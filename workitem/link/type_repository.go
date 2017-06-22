package link

import (
	"fmt"
	"time"

	"context"

	"github.com/almighty/almighty-core/application/repository"
	"github.com/almighty/almighty-core/errors"
	"github.com/almighty/almighty-core/log"
	"github.com/almighty/almighty-core/space"
	"github.com/almighty/almighty-core/workitem"

	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// WorkItemLinkTypeRepository encapsulates storage & retrieval of work item link types
type WorkItemLinkTypeRepository interface {
	repository.Exister
	Create(ctx context.Context, linkType *WorkItemLinkType) (*WorkItemLinkType, error)
	Load(ctx context.Context, ID uuid.UUID) (*WorkItemLinkType, error)
	List(ctx context.Context, spaceID uuid.UUID) ([]WorkItemLinkType, error)
	Delete(ctx context.Context, spaceID uuid.UUID, ID uuid.UUID) error
	Save(ctx context.Context, linkCat WorkItemLinkType) (*WorkItemLinkType, error)
	// ListSourceLinkTypes returns the possible link types for where the given
	// WIT can be used in the source.
	ListSourceLinkTypes(ctx context.Context, witID uuid.UUID) ([]WorkItemLinkType, error)
	// ListSourceLinkTypes returns the possible link types for where the given
	// WIT can be used in the target.
	ListTargetLinkTypes(ctx context.Context, witID uuid.UUID) ([]WorkItemLinkType, error)
}

// NewWorkItemLinkTypeRepository creates a work item link type repository based on gorm
func NewWorkItemLinkTypeRepository(db *gorm.DB) *GormWorkItemLinkTypeRepository {
	return &GormWorkItemLinkTypeRepository{db}
}

// GormWorkItemLinkTypeRepository implements WorkItemLinkTypeRepository using gorm
type GormWorkItemLinkTypeRepository struct {
	db *gorm.DB
}

// Create creates a new work item link type in the repository.
// Returns BadParameterError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Create(ctx context.Context, linkType *WorkItemLinkType) (*WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "create"}, time.Now())
	if err := linkType.CheckValidForCreation(); err != nil {
		return nil, errs.WithStack(err)
	}
	// Check link category exists
	linkCategory := WorkItemLinkCategory{}
	db := r.db.Where("id=?", linkType.LinkCategoryID).Find(&linkCategory)
	if db.RecordNotFound() {
		return nil, errors.NewBadParameterError("work item link category", linkType.LinkCategoryID)
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(ctx, errs.Wrap(db.Error, "failed to find work item link category"))
	}
	// Check space exists
	space := space.Space{}
	db = r.db.Where("id=?", linkType.SpaceID).Find(&space)
	if db.RecordNotFound() {
		return nil, errors.NewBadParameterError("work item link space", linkType.SpaceID)
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(ctx, errs.Wrap(db.Error, "failed to find work item link space"))
	}

	db = r.db.Create(linkType)
	if db.Error != nil {
		return nil, errors.NewInternalError(ctx, db.Error)
	}
	return linkType, nil
}

// Load returns the work item link type for the given ID.
// Returns NotFoundError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Load(ctx context.Context, ID uuid.UUID) (*WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "load"}, time.Now())
	log.Info(ctx, map[string]interface{}{
		"wilt_id": ID,
	}, "loading work item link type")
	modelLinkType := WorkItemLinkType{}
	db := r.db.Model(&modelLinkType).Where("id=?", ID).First(&modelLinkType)
	if db.RecordNotFound() {
		log.Error(ctx, map[string]interface{}{
			"wilt_id": ID,
		}, "work item link type not found")
		return nil, errors.NewNotFoundError("work item link type", ID.String())
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(ctx, db.Error)
	}
	return &modelLinkType, nil
}

// Exists returns true|false whether a work item link type exists with a specific identifier
func (m *GormWorkItemLinkTypeRepository) Exists(ctx context.Context, id string) (bool, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "exists"}, time.Now())
	return repository.Exists(ctx, m.db, WorkItemLinkType{}.TableName(), id)
}

// List returns all work item link types
// TODO: Handle pagination
func (r *GormWorkItemLinkTypeRepository) List(ctx context.Context, spaceID uuid.UUID) ([]WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "list"}, time.Now())
	log.Info(ctx, map[string]interface{}{
		"space_id": spaceID,
	}, "Listing work item link types by space ID %s", spaceID.String())

	// We don't have any where clause or paging at the moment.
	var modelLinkTypes []WorkItemLinkType
	db := r.db.Where("space_id = ?", spaceID)
	if err := db.Find(&modelLinkTypes).Error; err != nil {
		return nil, errs.WithStack(err)
	}
	return modelLinkTypes, nil
}

// Delete deletes the work item link type with the given id
// returns NotFoundError or InternalError
func (r *GormWorkItemLinkTypeRepository) Delete(ctx context.Context, spaceID uuid.UUID, ID uuid.UUID) error {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "delete"}, time.Now())
	var cat = WorkItemLinkType{
		ID:      ID,
		SpaceID: spaceID,
	}
	log.Info(ctx, map[string]interface{}{
		"wilt_id":  ID,
		"space_id": spaceID,
	}, "Work item link type to delete %v", cat)

	db := r.db.Delete(&cat)
	if db.Error != nil {
		return errors.NewInternalError(ctx, db.Error)
	}
	if db.RowsAffected == 0 {
		return errors.NewNotFoundError("work item link type", ID.String())
	}
	return nil
}

// Save updates the given work item link type in storage. Version must be the same as the one int the stored version.
// returns NotFoundError, VersionConflictError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Save(ctx context.Context, modelToSave WorkItemLinkType) (*WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "save"}, time.Now())
	existingModel := WorkItemLinkType{}
	db := r.db.Model(&existingModel).Where("id=?", modelToSave.ID).First(&existingModel)
	if db.RecordNotFound() {
		log.Error(ctx, map[string]interface{}{
			"wilt_id": modelToSave.ID,
		}, "work item link type not found")
		return nil, errors.NewNotFoundError("work item link type", modelToSave.ID.String())
	}
	if db.Error != nil {
		log.Error(ctx, map[string]interface{}{
			"wilt_id": modelToSave.ID,
			"err":     db.Error,
		}, "unable to find work item link type repository")
		return nil, errors.NewInternalError(ctx, db.Error)
	}
	if existingModel.Version != modelToSave.Version {
		return nil, errors.NewVersionConflictError("version conflict")
	}
	modelToSave.Version = modelToSave.Version + 1
	db = db.Save(&modelToSave)
	if db.Error != nil {
		log.Error(ctx, map[string]interface{}{
			"wilt_id": existingModel.ID,
			"wilt":    existingModel,
			"err":     db.Error,
		}, "unable to save work item link type repository")
		return nil, errors.NewInternalError(ctx, db.Error)
	}
	log.Info(ctx, map[string]interface{}{
		"wilt_id": existingModel.ID,
		"wilt":    existingModel,
	}, "Work item link type updated %v", modelToSave)
	return &modelToSave, nil
}

func (r *GormWorkItemLinkTypeRepository) ListSourceLinkTypes(ctx context.Context, witID uuid.UUID) ([]WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "listSourceLinkTypes"}, time.Now())
	db := r.db.Model(WorkItemLinkType{})
	query := fmt.Sprintf(`
			-- Get link types we can use with a specific WIT if the WIT is at the
			-- source of the link.
			(SELECT path FROM %[2]s WHERE id = %[1]s.source_type_id LIMIT 1)
			@>
			(SELECT path FROM %[2]s WHERE id = ? LIMIT 1)`,
		WorkItemLinkType{}.TableName(),
		workitem.WorkItemType{}.TableName(),
	)
	db = db.Where(query, witID)
	var rows []WorkItemLinkType
	db = db.Find(&rows)
	if db.RecordNotFound() {
		return nil, nil
	}
	if db.Error != nil {
		return nil, errs.WithStack(db.Error)
	}
	return rows, nil
}

func (r *GormWorkItemLinkTypeRepository) ListTargetLinkTypes(ctx context.Context, witID uuid.UUID) ([]WorkItemLinkType, error) {
	defer goa.MeasureSince([]string{"goa", "db", "workitemlinktype", "listTargetLinkTypes"}, time.Now())
	db := r.db.Model(WorkItemLinkType{})
	query := fmt.Sprintf(`
			-- Get link types we can use with a specific WIT if the WIT is at the
			-- target of the link.
			(SELECT path FROM %[2]s WHERE id = %[1]s.target_type_id LIMIT 1)
			@>
			(SELECT path FROM %[2]s WHERE id = ? LIMIT 1)`,
		WorkItemLinkType{}.TableName(),
		workitem.WorkItemType{}.TableName(),
	)
	db = db.Where(query, witID)
	var rows []WorkItemLinkType
	db = db.Find(&rows)
	if db.RecordNotFound() {
		return nil, nil
	}
	if db.Error != nil {
		return nil, errs.WithStack(db.Error)
	}
	return rows, nil
}
