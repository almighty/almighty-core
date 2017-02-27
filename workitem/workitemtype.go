package workitem

import (
	"strconv"
	"strings"

	"github.com/almighty/almighty-core/app"
	"github.com/almighty/almighty-core/convert"
	"github.com/almighty/almighty-core/gormsupport"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// String constants for the local work item types.
const (
	// pathSep specifies the symbol used to concatenate WIT names to form a so called "path"
	pathSep = "."

	SystemRemoteItemID        = "system.remote_item_id"
	SystemTitle               = "system.title"
	SystemDescription         = "system.description"
	SystemDescriptionMarkup   = "system.description.markup"
	SystemDescriptionRendered = "system.description.rendered"
	SystemState               = "system.state"
	SystemAssignees           = "system.assignees"
	SystemCreator             = "system.creator"
	SystemCreatedAt           = "system.created_at"
	SystemIteration           = "system.iteration"
	SystemArea                = "system.area"

	SystemStateOpen       = "open"
	SystemStateNew        = "new"
	SystemStateInProgress = "in progress"
	SystemStateResolved   = "resolved"
	SystemStateClosed     = "closed"
)

// Never ever change these UUIDs!!!
var (
	// base item type with common fields for planner item types like userstory, experience, bug, feature, etc.
	SystemPlannerItem      = uuid.FromStringOrNil("86af5178-9b41-469b-9096-57e5155c3f31") // "planneritem"
	SystemUserStory        = uuid.FromStringOrNil("bbf35418-04b6-426c-a60b-7f80beb0b624") // "userstory"
	SystemValueProposition = uuid.FromStringOrNil("3194ab60-855b-4155-9005-9dce4a05f1eb") // "valueproposition"
	SystemFundamental      = uuid.FromStringOrNil("ee7ca005-f81d-4eea-9b9b-1965df0988d0") // "fundamental"
	SystemExperience       = uuid.FromStringOrNil("b9a71831-c803-4f66-8774-4193fffd1311") // "experience"
	SystemFeature          = uuid.FromStringOrNil("0a24d3c2-e0a6-4686-8051-ec0ea1915a28") // "feature"
	SystemScenario         = uuid.FromStringOrNil("71171e90-6d35-498f-a6a7-2083b5267c18") // "scenario"
	SystemBug              = uuid.FromStringOrNil("26787039-b68f-4e28-8814-c2f93be1ef4e") // "bug"
)

// WorkItemType represents a work item type as it is stored in the db
type WorkItemType struct {
	gormsupport.Lifecycle
	// the unique name of this work item type.
	Name string `gorm:"primary_key"`
	// Version for optimistic concurrency control
	Version int
	// the id's of the parents, separated with some separator
	Path string
	// definitions of the fields this work item type supports
	Fields FieldDefinitions `sql:"type:jsonb"`
}

// GetTypePathSeparator returns the work item type's path separator "."
func GetTypePathSeparator() string {
	return pathSep
}

// TableName implements gorm.tabler
func (wit WorkItemType) TableName() string {
	return "work_item_types"
}

// Ensure Fields implements the Equaler interface
var _ convert.Equaler = WorkItemType{}
var _ convert.Equaler = (*WorkItemType)(nil)

// Equal returns true if two WorkItemType objects are equal; otherwise false is returned.
func (wit WorkItemType) Equal(u convert.Equaler) bool {
	other, ok := u.(WorkItemType)
	if !ok {
		return false
	}
	if !wit.Lifecycle.Equal(other.Lifecycle) {
		return false
	}
	if wit.Version != other.Version {
		return false
	}
	if wit.Name != other.Name {
		return false
	}
	if wit.Path != other.Path {
		return false
	}
	if len(wit.Fields) != len(other.Fields) {
		return false
	}
	for witKey, witVal := range wit.Fields {
		otherVal, keyFound := other.Fields[witKey]
		if !keyFound {
			return false
		}
		if !witVal.Equal(otherVal) {
			return false
		}
	}
	return true
}

// ConvertFromModel converts a workItem from the persistence layer into a workItem of the API layer
func (wit WorkItemType) ConvertFromModel(workItem WorkItem) (*app.WorkItem, error) {
	result := app.WorkItem{
		ID:      strconv.FormatUint(workItem.ID, 10),
		Type:    workItem.Type,
		Version: workItem.Version,
		Fields:  map[string]interface{}{}}

	for name, field := range wit.Fields {
		var err error
		if name == SystemCreatedAt {
			continue
		}
		result.Fields[name], err = field.ConvertFromModel(name, workItem.Fields[name])
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &result, nil
}

// IsTypeOrSubtypeOf returns true if the work item type is of the given type name,
// or a subtype; otherwise false is returned.
func (wit WorkItemType) IsTypeOrSubtypeOf(typeName string) bool {
	// Remove any prefixed "."
	for strings.HasPrefix(typeName, pathSep) && len(typeName) > 0 {
		typeName = strings.TrimPrefix(typeName, pathSep)
	}
	// Remove any trailing "."
	for strings.HasSuffix(typeName, pathSep) && len(typeName) > 0 {
		typeName = strings.TrimSuffix(typeName, pathSep)
	}
	if len(typeName) <= 0 {
		return false
	}
	// Check for complete inclusion (e.g. "bar" is contained in "foo.bar.cake")
	// and for suffix (e.g. ".cake" is the suffix of "foo.bar.cake").
	return wit.Name == typeName || strings.Contains(wit.Path, typeName+pathSep)
}
