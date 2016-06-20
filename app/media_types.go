//************************************************************************//
// API "alm": Application Media Types
//
// Generated with goagen v0.0.1, command line:
// $ goagen.exe
// --design=github.com/ALMighty/almighty-core/design
// --out=$(GOPATH)\src\github.com\ALMighty\almighty-core
//
// The content of this file is auto-generated, DO NOT MODIFY
//************************************************************************//

package app

import "github.com/goadesign/goa"

// AuthToken media type.
//
// Identifier: application/vnd.authtoken+json
type AuthToken struct {
	// JWT Token
	Token string `json:"token" xml:"token" form:"token"`
}

// Validate validates the AuthToken media type instance.
func (mt *AuthToken) Validate() (err error) {
	if mt.Token == "" {
		err = goa.MergeErrors(err, goa.MissingAttributeError(`response`, "token"))
	}

	return
}

// Version media type.
//
// Identifier: application/vnd.version+json
type Version struct {
	// The date when build
	BuildTime string `json:"build_time" xml:"build_time" form:"build_time"`
	// Commit SHA this build is based on
	Commit string `json:"commit" xml:"commit" form:"commit"`
}

// Validate validates the Version media type instance.
func (mt *Version) Validate() (err error) {
	if mt.Commit == "" {
		err = goa.MergeErrors(err, goa.MissingAttributeError(`response`, "commit"))
	}
	if mt.BuildTime == "" {
		err = goa.MergeErrors(err, goa.MissingAttributeError(`response`, "build_time"))
	}

	return
}
