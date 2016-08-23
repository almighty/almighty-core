package design

import (
	. "github.com/goadesign/goa/design"
	. "github.com/goadesign/goa/design/apidsl"
)

// ALMVersion defines the running ALM Version MediaType
var ALMVersion = MediaType("application/vnd.version+json", func() {
	Description("The current running version")
	Attributes(func() {
		Attribute("commit", String, "Commit SHA this build is based on")
		Attribute("build_time", String, "The date when build")
		Required("commit", "build_time")
	})
	View("default", func() {
		Attribute("commit")
		Attribute("build_time")
	})
})

// AuthToken represents an authentication JWT Token
var AuthToken = MediaType("application/vnd.authtoken+json", func() {
	TypeName("AuthToken")
	Description("JWT Token")
	Attributes(func() {
		Attribute("token", String, "JWT Token")
		Required("token")
	})
	View("default", func() {
		Attribute("token")
	})
})

var WorkItem = MediaType("application/vnd.workitem+json", func() {
	TypeName("WorkItem")
	Description("ALM Work Item")
	Attribute("id", String, "unique id per installation")
	Attribute("version", Integer, "Version for optimistic concurrency control")
	Attribute("name", String, "User Readable Name of this item")
	Attribute("type", String, "Id of the type of this work item")
	Attribute("fields", HashOf(String, Any))

	Required("id")
	Required("version")
	Required("name")
	Required("type")
	Required("fields")

	View("default", func() {
		Attribute("id")
		Attribute("version")
		Attribute("name")
		Attribute("type")
		Attribute("fields")
	})
})

var FieldDefinition = Type("fieldDefinition", func() {
	Attribute("required", Boolean)
	Attribute("type", FieldType)

	Required("required")
	Required("type")

	View("default", func() {
		Attribute("kind")
	})
})

var FieldType = Type("fieldType", func() {

	Attribute("kind", String)
	Attribute("componentType", String)
	Attribute("baseType", String)
	Attribute("values", ArrayOf(Any))

	Required("kind")
})

var WorkItemType = MediaType("application/vnd.workitemtype+json", func() {
	TypeName("WorkItemType")
	Description("ALM Work Item Type")
	Attribute("version", Integer, "Version for optimistic concurrency control")
	Attribute("name", String, "User Readable Name of this item")
	Attribute("parentPath", String, "The ID's of the parents, separated with some separator")
	Attribute("fields", HashOf(String, FieldDefinition), "Definitions of fields in this work item")

	Required("version")
	Required("name")
	Required("fields")

	View("default", func() {
		Attribute("version")
		Attribute("name")
		Attribute("fields")
		Attribute("parentPath")
	})
	View("link", func() {
		Attribute("name")
	})

})
