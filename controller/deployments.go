package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/fabric8-services/fabric8-wit/account"
	"github.com/fabric8-services/fabric8-wit/app"
	"github.com/fabric8-services/fabric8-wit/configuration"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/jsonapi"
	"github.com/fabric8-services/fabric8-wit/kubernetes"
	"github.com/fabric8-services/fabric8-wit/log"

	"github.com/goadesign/goa"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

/*
 The deployments API can operate directly talking to Auth and OSO,
 or it can use the fabric8-tenant service as a replacement for fabric8-auth
 and the fabric8-froxy service as a replacement for direct OpenShift API calls.int64

 If the tenant service is available, it will be used in place of auth.
 If the proxy service is available, it will be used in place of direct calls.
*/

// DeploymentsController implements the deployments resource.
type DeploymentsController struct {
	*goa.Controller
	Config *configuration.Registry
	ClientGetter
}

// ClientGetter creates an instances of clients used by this controller
type ClientGetter interface {
	GetKubeClient(ctx context.Context) (kubernetes.KubeClientInterface, error)
	GetAndCheckOSIOClient(ctx context.Context) (OpenshiftIOClient, error)
}

// Default implementation of KubeClientGetter and OSIOClientGetter used by NewDeploymentsController
type defaultClientGetter struct {
	config            *configuration.Registry
	OpenshiftProxyURL string
	TenantURL         *string
}

// NewDeploymentsController creates a deployments controller.
func NewDeploymentsController(service *goa.Service, config *configuration.Registry) *DeploymentsController {
	osproxy := config.GetOpenshiftProxyURL()
	//tenant := config.GetTenantServiceURL()
	//tenantURL := &tenant
	//if len(tenant) == 0 {
	//	tenantURL = nil
	//}
	return &DeploymentsController{
		Controller: service.NewController("DeploymentsController"),
		Config:     config,
		ClientGetter: &defaultClientGetter{
			config:            config,
			OpenshiftProxyURL: osproxy,
			//TenantURL:           tenantURL,
		},
	}
}

func tostring(item interface{}) string {
	bytes, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func (*defaultClientGetter) GetAndCheckOSIOClient(ctx context.Context) (OpenshiftIOClient, error) {

	// defaults
	host := "localhost"
	scheme := "https"

	req := goa.ContextRequest(ctx)
	if req != nil {
		// Note - it's probably more efficient to force a loopback host, and only use the port number here
		// (on some systems using a non-loopback interface forces a network stack traverse)
		host = req.Host
		scheme = req.URL.Scheme
	}

	// The deployments API communicates with the rest of WIT via the stnadard WIT API.
	// This environment variable is used for local development of the deployments API, to point ot a remote WIT.
	witURLStr := os.Getenv("FABRIC8_WIT_API_URL")
	if witURLStr != "" {
		witurl, err := url.Parse(witURLStr)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"FABRIC8_WIT_API_URL": witURLStr,
				"err": err,
			}, "cannot parse FABRIC8_WIT_API_URL: %s", witURLStr)
			return nil, errs.Wrapf(err, "cannot parse FABRIC8_WIT_API_URL: %s", witURLStr)
		}
		host = witurl.Host
		scheme = witurl.Scheme
	}

	oc := NewOSIOClient(ctx, scheme, host)

	return oc, nil
}

// getSpaceNameFromSpaceID() converts an OSIO Space UUID to an OpenShift space name.
// will return an error if the space is not found.
func (c *DeploymentsController) getSpaceNameFromSpaceID(ctx context.Context, spaceID uuid.UUID) (*string, error) {
	// TODO - add a cache in DeploymentsController - but will break if user can change space name
	// use WIT API to convert Space UUID to Space name
	osioclient, err := c.GetAndCheckOSIOClient(ctx)
	if err != nil {
		return nil, err
	}

	osioSpace, err := osioclient.GetSpaceByID(ctx, spaceID)
	if err != nil {
		return nil, errs.Wrapf(err, "unable to convert space UUID %s to space name", spaceID)
	}
	if osioSpace == nil || osioSpace.Attributes == nil || osioSpace.Attributes.Name == nil {
		return nil, errs.Errorf("space UUID %s is not valid space name", spaceID)
	}
	return osioSpace.Attributes.Name, nil
}

func (g *defaultClientGetter) getNamespaceName(ctx context.Context) (*string, error) {

	osioclient, err := g.GetAndCheckOSIOClient(ctx)
	if err != nil {
		return nil, err
	}

	kubeSpaceAttr, err := osioclient.GetNamespaceByType(ctx, nil, "user")
	if err != nil {
		return nil, errs.Wrap(err, "unable to retrieve 'user' namespace")
	}
	if kubeSpaceAttr == nil {
		return nil, errors.NewNotFoundError("namespace", "user")
	}

	return kubeSpaceAttr.Name, nil
}

// GetKubeClient creates a kube client for the appropriate cluster assigned to the current user
func (g *defaultClientGetter) GetKubeClient(ctx context.Context) (kubernetes.KubeClientInterface, error) {

	kubeURL := g.OpenshiftProxyURL

	if g.TenantURL != nil {
		tenant, err := account.ShowTenant(ctx, g.config)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err": err,
			}, "error accessing Tenant server")
			return nil, errs.Wrapf(err, "error creating Tenant client")
		}

		fmt.Printf("tenant = %s\n", tostring(tenant.Data.Attributes))

	}

	baseURLProvider, err := kubernetes.NewURLProvider(ctx, g.config)

	kubeNamespaceName, err := g.getNamespaceName(ctx)
	if err != nil {
		return nil, errs.Wrap(err, "could not retrieve namespace name")
	}

	// create the cluster API client
	kubeConfig := &kubernetes.KubeClientConfig{
		BaseURLProvider: baseURLProvider,
		UserNamespace:   *kubeNamespaceName,
	}
	kc, err := kubernetes.NewKubeClient(kubeConfig)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":            err,
			"user_namespace": *kubeNamespaceName,
			"cluster":        kubeURL,
		}, "could not create Kubernetes client object")
		return nil, errs.Wrap(err, "could not create Kubernetes client object")
	}
	return kc, nil
}

// SetDeployment runs the setDeployment action.
func (c *DeploymentsController) SetDeployment(ctx *app.SetDeploymentDeploymentsContext) error {

	// we double check podcount here, because in the future we might have different query parameters
	// (for setting different Pod switches) and PodCount might become optional
	if ctx.PodCount == nil {
		return errors.NewBadParameterError("podCount", "missing")
	}

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return errors.NewNotFoundError("osio space", ctx.SpaceID.String())
	}

	_ /*oldCount*/, err = kc.ScaleDeployment(*kubeSpaceName, ctx.AppName, ctx.DeployName, *ctx.PodCount)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "error scaling deployment %s", ctx.DeployName))
	}

	return ctx.OK([]byte{})
}

// DeleteDeployment runs the deleteDeployment action.
func (c *DeploymentsController) DeleteDeployment(ctx *app.DeleteDeploymentDeploymentsContext) error {
	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, goa.ErrUnauthorized(err.Error()))
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, goa.ErrNotFound(err.Error()))
	}

	err = kc.DeleteDeployment(*kubeSpaceName, ctx.AppName, ctx.DeployName)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":        err,
			"space_name": *kubeSpaceName,
		}, "error deleting deployment")
		return jsonapi.JSONErrorResponse(ctx, goa.ErrInternal(err.Error()))
	}

	return ctx.OK([]byte{})
}

// ShowDeploymentStatSeries runs the showDeploymentStatSeries action.
func (c *DeploymentsController) ShowDeploymentStatSeries(ctx *app.ShowDeploymentStatSeriesDeploymentsContext) error {

	endTime := time.Now()
	startTime := endTime.Add(-8 * time.Hour) // default: start time is 8 hours before end time
	limit := -1                              // default: No limit

	if ctx.Limit != nil {
		limit = *ctx.Limit
	}

	if ctx.Start != nil {
		startTime = convertToTime(int64(*ctx.Start))
	}

	if ctx.End != nil {
		endTime = convertToTime(int64(*ctx.End))
	}

	if endTime.Before(startTime) {
		return errors.NewBadParameterError("end", *ctx.End)
	}

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return err
	}

	statSeries, err := kc.GetDeploymentStatSeries(*kubeSpaceName, ctx.AppName, ctx.DeployName,
		startTime, endTime, limit)
	if err != nil {
		return err
	} else if statSeries == nil {
		return errors.NewNotFoundError("deployment", ctx.DeployName)
	}

	res := &app.SimpleDeploymentStatSeriesSingle{
		Data: statSeries,
	}

	return ctx.OK(res)
}

func convertToTime(unixMillis int64) time.Time {
	return time.Unix(0, unixMillis*int64(time.Millisecond))
}

// ShowDeploymentStats runs the showDeploymentStats action.
func (c *DeploymentsController) ShowDeploymentStats(ctx *app.ShowDeploymentStatsDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return errors.NewNotFoundError("osio space", ctx.SpaceID.String())
	}

	var startTime time.Time
	if ctx.Start != nil {
		startTime = convertToTime(int64(*ctx.Start))
	} else {
		// If a start time was not supplied, default to one minute ago
		startTime = time.Now().Add(-1 * time.Minute)
	}

	deploymentStats, err := kc.GetDeploymentStats(*kubeSpaceName, ctx.AppName, ctx.DeployName, startTime)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "could not retrieve deployment statistics for %s", ctx.DeployName))
	}
	if deploymentStats == nil {
		return errors.NewNotFoundError("deployment", ctx.DeployName)
	}

	deploymentStats.ID = ctx.DeployName

	res := &app.SimpleDeploymentStatsSingle{
		Data: deploymentStats,
	}

	return ctx.OK(res)
}

// ShowEnvironment runs the showEnvironment action.
func (c *DeploymentsController) ShowEnvironment(ctx *app.ShowEnvironmentDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	env, err := kc.GetEnvironment(ctx.EnvName)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "could not retrieve environment %s", ctx.EnvName))
	}
	if env == nil {
		return errors.NewNotFoundError("environment", ctx.EnvName)
	}

	env.ID = *env.Attributes.Name

	res := &app.SimpleEnvironmentSingle{
		Data: env,
	}

	return ctx.OK(res)
}

// ShowSpace runs the showSpace action.
func (c *DeploymentsController) ShowSpace(ctx *app.ShowSpaceDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil || kubeSpaceName == nil {
		return errors.NewNotFoundError("osio space", ctx.SpaceID.String())
	}

	// get OpenShift space
	space, err := kc.GetSpace(*kubeSpaceName)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "could not retrieve space %s", *kubeSpaceName))
	}
	if space == nil {
		return errors.NewNotFoundError("openshift space", *kubeSpaceName)
	}

	// Kubernetes doesn't know about space ID, so add it here
	space.ID = ctx.SpaceID

	res := &app.SimpleSpaceSingle{
		Data: space,
	}

	return ctx.OK(res)
}

// ShowSpaceApp runs the showSpaceApp action.
func (c *DeploymentsController) ShowSpaceApp(ctx *app.ShowSpaceAppDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return errors.NewNotFoundError("osio space", ctx.SpaceID.String())
	}

	theapp, err := kc.GetApplication(*kubeSpaceName, ctx.AppName)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "could not retrieve application %s", ctx.AppName))
	}
	if theapp == nil {
		return errors.NewNotFoundError("application", ctx.AppName)
	}

	theapp.ID = theapp.Attributes.Name

	res := &app.SimpleApplicationSingle{
		Data: theapp,
	}

	return ctx.OK(res)
}

// ShowSpaceAppDeployment runs the showSpaceAppDeployment action.
func (c *DeploymentsController) ShowSpaceAppDeployment(ctx *app.ShowSpaceAppDeploymentDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	kubeSpaceName, err := c.getSpaceNameFromSpaceID(ctx, ctx.SpaceID)
	if err != nil {
		return errors.NewNotFoundError("osio space", ctx.SpaceID.String())
	}

	deploymentStats, err := kc.GetDeployment(*kubeSpaceName, ctx.AppName, ctx.DeployName)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "error retrieving deployment %s", ctx.DeployName))
	}
	if deploymentStats == nil {
		return errors.NewNotFoundError("deployment statistics", ctx.DeployName)
	}

	deploymentStats.ID = deploymentStats.Attributes.Name

	res := &app.SimpleDeploymentSingle{
		Data: deploymentStats,
	}

	return ctx.OK(res)
}

// ShowEnvAppPods runs the showEnvAppPods action.
func (c *DeploymentsController) ShowEnvAppPods(ctx *app.ShowEnvAppPodsDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	pods, err := kc.GetPodsInNamespace(ctx.EnvName, ctx.AppName)
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrapf(err, "error retrieving pods from namespace %s/%s", ctx.EnvName, ctx.AppName))
	}
	if pods == nil || len(pods) == 0 {
		return errors.NewNotFoundError("pods", ctx.AppName)
	}

	jsonresp := fmt.Sprintf("{\"data\":{\"attributes\":{\"environment\":\"%s\",\"application\":\"%s\",\"pods\":%s}}}", ctx.EnvName, ctx.AppName, tostring(pods))

	return ctx.OK([]byte(jsonresp))
}

// ShowSpaceEnvironments runs the showSpaceEnvironments action.
func (c *DeploymentsController) ShowSpaceEnvironments(ctx *app.ShowSpaceEnvironmentsDeploymentsContext) error {

	kc, err := c.GetKubeClient(ctx)
	defer cleanup(kc)
	if err != nil {
		return errors.NewUnauthorizedError("openshift token")
	}

	envs, err := kc.GetEnvironments()
	if err != nil {
		return errors.NewInternalError(ctx, errs.Wrap(err, "error retrieving environments"))
	}
	if envs == nil {
		return errors.NewNotFoundError("environments", ctx.SpaceID.String())
	}

	res := &app.SimpleEnvironmentList{
		Data: envs,
	}

	return ctx.OK(res)
}

func cleanup(kc kubernetes.KubeClientInterface) {
	if kc != nil {
		kc.Close()
	}
}
