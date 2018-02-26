package workitem

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
)

// A TableJoin helps to construct a query like this:
//
//   SELECT *
//     FROM workitems
//     JOIN iterations iter ON fields@> concat('{"system.iteration": "', iter.ID, '"}')::jsonb
//     WHERE iter.name = "foo"
//
// With the prefix triggers we can identify if a certain field expression points
// at data from a joined table. By default there are no restrictions on what can
// be queried in the joined table but if you fill the allowed/disallowed columns
// arrays you can explicitly allow or disallow columns to be queried. The names
// in the allowed/disalowed columns are those of the table.
type TableJoin struct {
	active            bool     // true if this table join is used
	tableName         string   // e.g. "iterations"
	tableAlias        string   // e.g. "iter"
	on                string   // e.g. `fields@> concat('{"system.iteration": "', iter.ID, '"}')::jsonb`
	prefixActivators  []string // e.g. []string{"iteration."}
	allowedColumns    []string // e.g. ["name"]. When empty all columns are allowed.
	disallowedColumns []string // e.g. ["created_at"]. When empty all columns are allowed.
	handledFields     []string // e.g. []string{"name", "created_at", "foobar"}
	// TODO(kwk): Maybe introduce a column mapping table here: ColumnMapping map[string]string
}

// IsValid returns nil if the join is active and all the fields handled by this
// join do exist in the joined table; otherwise an error is returned.
func (j TableJoin) IsValid(db *gorm.DB) error {
	dialect := db.Dialect()
	dialect.SetDB(db.CommonDB())
	if j.IsActive() {
		for _, f := range j.handledFields {
			if !dialect.HasColumn(j.tableName, f) {
				return errs.Errorf(`table "%s" has no column "%s"`, j.tableName, f)
			}
		}
	}
	return nil
}

// Activate tells the search engine to actually use this join information;
// otherwise it won't be used.
func (j *TableJoin) Activate() {
	j.active = true
}

// IsActive returns true if this table join was activated; otherwise false is
// returned.
func (j TableJoin) IsActive() bool {
	return j.active
}

// JoinOnJSONField returns the ON part of an SQL JOIN for the given fields
func JoinOnJSONField(jsonField, foreignCol string) string {
	return fmt.Sprintf(`fields@> concat('{"%[1]s": "', %[2]s, '"}')::jsonb`, jsonField, foreignCol)
}

// String implements Stringer interface
func (j TableJoin) String() string {
	return "LEFT JOIN " + j.tableName + " " + j.tableAlias + " ON " + j.on
}

// HandlesFieldName returns true if the given field name should be handled by
// this table join.
func (j *TableJoin) HandlesFieldName(fieldName string) bool {
	for _, t := range j.prefixActivators {
		if strings.HasPrefix(fieldName, t) {
			return true
		}
	}
	return false
}

// TranslateFieldName returns a non-empty string if the given field name has the
// prefix specified by the table join and if the field is allowed to be queried;
// otherwise it returns an empty string.
func (j *TableJoin) TranslateFieldName(fieldName string) (string, error) {
	if !j.HandlesFieldName(fieldName) {
		return "", errs.Errorf(`field name "%s" not handled by this table join`, fieldName)
	}

	// Ensure this join is active
	j.Activate()

	var prefix string
	for _, t := range j.prefixActivators {
		if strings.HasPrefix(fieldName, t) {
			prefix = t
		}
	}
	col := strings.TrimPrefix(fieldName, prefix)
	col = strings.TrimSpace(col)
	if col == "" {
		return "", errs.Errorf(`field name "%s" contains an empty column name after prefix "%s"`, fieldName, prefix)
	}
	if strings.Contains(col, "'") {
		// beware of injection, it's a reasonable restriction for field names,
		// make sure it's not allowed when creating wi types
		return "", errs.Errorf(`single quote not allowed in field name: "%s"`, col)
	}

	// now we have the final column name

	// if no columns are explicitly allowed, then this column is allowed by
	// default.
	columnIsAllowed := (j.allowedColumns == nil || len(j.allowedColumns) == 0)
	for _, c := range j.allowedColumns {
		if c == col {
			columnIsAllowed = true
			break
		}
	}
	// check if a column is explicitly disallowed
	for _, c := range j.disallowedColumns {
		if c == col {
			columnIsAllowed = false
			break
		}
	}
	if !columnIsAllowed {
		return "", errs.Errorf("column is not allowed: %s", col)
	}
	j.handledFields = append(j.handledFields, col)
	return j.tableAlias + "." + col, nil
}
