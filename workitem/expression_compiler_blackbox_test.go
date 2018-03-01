package workitem_test

import (
	"testing"

	c "github.com/fabric8-services/fabric8-wit/criteria"
	"github.com/fabric8-services/fabric8-wit/resource"
	"github.com/fabric8-services/fabric8-wit/workitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestField(t *testing.T) {
	t.Parallel()
	resource.Require(t, resource.UnitTest)
	wiTbl := workitem.WorkItemStorage{}.TableName()
	expect(t, c.Equals(c.Field("foo.bar"), c.Literal(23)), `(`+workitem.Column(wiTbl, "fields")+` @> '{"foo.bar" : 23}')`, []interface{}{}, nil)
	expect(t, c.Equals(c.Field("foo"), c.Literal(23)), `(`+workitem.Column(wiTbl, "foo")+` = ?)`, []interface{}{23}, nil)
	expect(t, c.Equals(c.Field("Type"), c.Literal("abcd")), `(`+workitem.Column(wiTbl, "type")+` = ?)`, []interface{}{"abcd"}, nil)
	expect(t, c.Not(c.Field("Type"), c.Literal("abcd")), `(`+workitem.Column(wiTbl, "type")+` != ?)`, []interface{}{"abcd"}, nil)
	expect(t, c.Not(c.Field("Version"), c.Literal("abcd")), `(`+workitem.Column(wiTbl, "version")+` != ?)`, []interface{}{"abcd"}, nil)
	expect(t, c.Not(c.Field("Number"), c.Literal("abcd")), `(`+workitem.Column(wiTbl, "number")+` != ?)`, []interface{}{"abcd"}, nil)
	expect(t, c.Not(c.Field("SpaceID"), c.Literal("abcd")), `(`+workitem.Column(wiTbl, "space_id")+` != ?)`, []interface{}{"abcd"}, nil)

	t.Run("test join", func(t *testing.T) {
		expect(t, c.Equals(c.Field("iteration.name"), c.Literal("abcd")), `(`+workitem.Column("iter", "name")+` = ?)`, []interface{}{"abcd"}, []string{"iteration"})
		expect(t, c.Equals(c.Field("area.name"), c.Literal("abcd")), `(`+workitem.Column("ar", "name")+` = ?)`, []interface{}{"abcd"}, []string{"area"})
		expect(t, c.Equals(c.Field("codebase.url"), c.Literal("abcd")), `(`+workitem.Column("cb", "url")+` = ?)`, []interface{}{"abcd"}, []string{"codebase"})
		expect(t, c.Equals(c.Field("wit.name"), c.Literal("abcd")), `(`+workitem.Column("wit", "name")+` = ?)`, []interface{}{"abcd"}, []string{"work_item_type"})
		expect(t, c.Equals(c.Field("work_item_type.name"), c.Literal("abcd")), `(`+workitem.Column("wit", "name")+` = ?)`, []interface{}{"abcd"}, []string{"work_item_type"})
		expect(t, c.Equals(c.Field("type.name"), c.Literal("abcd")), `(`+workitem.Column("wit", "name")+` = ?)`, []interface{}{"abcd"}, []string{"work_item_type"})
		expect(t, c.Equals(c.Field("space.name"), c.Literal("abcd")), `(`+workitem.Column("space", "name")+` = ?)`, []interface{}{"abcd"}, []string{"space"})
		expect(t, c.Equals(c.Field("creator.full_name"), c.Literal("abcd")), `(`+workitem.Column("creator", "full_name")+` = ?)`, []interface{}{"abcd"}, []string{"creator"})
		expect(t, c.Equals(c.Field("author.full_name"), c.Literal("abcd")), `(`+workitem.Column("creator", "full_name")+` = ?)`, []interface{}{"abcd"}, []string{"creator"})
		expect(t, c.Not(c.Field("author.full_name"), c.Literal("abcd")), `(`+workitem.Column("creator", "full_name")+` != ?)`, []interface{}{"abcd"}, []string{"creator"})

		expect(t, c.Or(
			c.Equals(c.Field("iteration.name"), c.Literal("abcd")),
			c.Equals(c.Field("area.name"), c.Literal("xyz")),
		), `((`+workitem.Column("iter", "name")+` = ?) OR (`+workitem.Column("ar", "name")+` = ?))`, []interface{}{"abcd", "xyz"}, []string{"iteration", "area"})

		expect(t, c.Or(
			c.Equals(c.Field("iteration.name"), c.Literal("abcd")),
			c.Equals(c.Field("iteration.created_at"), c.Literal("123")),
		), `((`+workitem.Column("iter", "name")+` = ?) OR (`+workitem.Column("iter", "created_at")+` = ?))`, []interface{}{"abcd", "123"}, []string{"iteration"})
	})
	t.Run("test illegal field name", func(t *testing.T) {
		t.Run("double quote", func(t *testing.T) {
			_, _, _, compileErrors := workitem.Compile(c.Equals(c.Field(`foo"bar`), c.Literal(23)))
			require.NotEmpty(t, compileErrors)
			require.Contains(t, compileErrors[0].Error(), "field name must not contain double quotes")
		})
		t.Run("single quote", func(t *testing.T) {
			_, _, _, compileErrors := workitem.Compile(c.Equals(c.Field(`foo'bar`), c.Literal(23)))
			require.NotEmpty(t, compileErrors)
			require.Contains(t, compileErrors[0].Error(), "field name must not contain single quotes")
		})
	})
}

func TestAndOr(t *testing.T) {
	t.Parallel()
	resource.Require(t, resource.UnitTest)
	expect(t, c.Or(c.Literal(true), c.Literal(false)), "(? OR ?)", []interface{}{true, false}, nil)

	wiTbl := workitem.WorkItemStorage{}.TableName()

	expect(t, c.And(c.Not(c.Field("foo.bar"), c.Literal("abcd")), c.Not(c.Literal(true), c.Literal(false))), `(NOT (`+workitem.Column(wiTbl, "fields")+` @> '{"foo.bar" : "abcd"}') AND (? != ?))`, []interface{}{true, false}, nil)
	expect(t, c.And(c.Equals(c.Field("foo.bar"), c.Literal("abcd")), c.Equals(c.Literal(true), c.Literal(false))), `((`+workitem.Column(wiTbl, "fields")+` @> '{"foo.bar" : "abcd"}') AND (? = ?))`, []interface{}{true, false}, nil)
	expect(t, c.Or(c.Equals(c.Field("foo.bar"), c.Literal("abcd")), c.Equals(c.Literal(true), c.Literal(false))), `((`+workitem.Column(wiTbl, "fields")+` @> '{"foo.bar" : "abcd"}') OR (? = ?))`, []interface{}{true, false}, nil)
}

func TestIsNull(t *testing.T) {
	t.Parallel()
	resource.Require(t, resource.UnitTest)
	wiTbl := workitem.WorkItemStorage{}.TableName()
	expect(t, c.IsNull("system.assignees"), `(`+workitem.Column(wiTbl, "fields")+`->>'system.assignees' IS NULL)`, []interface{}{}, nil)
	expect(t, c.IsNull("ID"), `(`+workitem.Column(wiTbl, "id")+` IS NULL)`, []interface{}{}, nil)
	expect(t, c.IsNull("Type"), `(`+workitem.Column(wiTbl, "type")+` IS NULL)`, []interface{}{}, nil)
	expect(t, c.IsNull("Version"), `(`+workitem.Column(wiTbl, "version")+` IS NULL)`, []interface{}{}, nil)
	expect(t, c.IsNull("Number"), `(`+workitem.Column(wiTbl, "number")+` IS NULL)`, []interface{}{}, nil)
	expect(t, c.IsNull("SpaceID"), `(`+workitem.Column(wiTbl, "space_id")+` IS NULL)`, []interface{}{}, nil)
}

func expect(t *testing.T, expr c.Expression, expectedClause string, expectedParameters []interface{}, expectedJoins []string) {
	clause, parameters, joins, compileErrors := workitem.Compile(expr)
	t.Run(expectedClause, func(t *testing.T) {
		t.Run("check for compile errors", func(t *testing.T) {
			require.Empty(t, compileErrors, "compile error")
		})
		t.Run("check clause", func(t *testing.T) {
			require.Equal(t, expectedClause, clause, "clause mismatch")
		})
		t.Run("check parameters", func(t *testing.T) {
			require.Equal(t, expectedParameters, parameters, "parameters mismatch")
		})
		t.Run("check joins", func(t *testing.T) {
			for _, k := range expectedJoins {
				_, ok := joins[k]
				require.True(t, ok, `joins is missing "%s"`)
			}
		})
	})
}

func TestArray(t *testing.T) {
	assignees := []string{"1", "2", "3"}

	exp := c.Equals(c.Field("system.assignees"), c.Literal(assignees))
	where, _, _, compileErrors := workitem.Compile(exp)
	require.Empty(t, compileErrors)
	wiTbl := workitem.WorkItemStorage{}.TableName()
	assert.Equal(t, `(`+workitem.Column(wiTbl, "fields")+` @> '{"system.assignees" : ["1","2","3"]}')`, where)
}

func TestSubstring(t *testing.T) {
	wiTbl := workitem.WorkItemStorage{}.TableName()
	t.Run("system.title with simple text", func(t *testing.T) {
		title := "some title"

		exp := c.Substring(c.Field("system.title"), c.Literal(title))
		where, _, _, compileErrors := workitem.Compile(exp)
		require.Empty(t, compileErrors)

		assert.Equal(t, workitem.Column(wiTbl, "fields")+`->>'system.title' ILIKE ?`, where)
	})
	t.Run("system.title with SQL injection text", func(t *testing.T) {
		title := "some title"

		exp := c.Substring(c.Field("system.title;DELETE FROM work_items"), c.Literal(title))
		where, _, _, compileErrors := workitem.Compile(exp)
		require.Empty(t, compileErrors)

		assert.Equal(t, workitem.Column(wiTbl, "fields")+`->>'system.title;DELETE FROM work_items' ILIKE ?`, where)
	})

	t.Run("system.title with SQL injection text single quote", func(t *testing.T) {
		title := "some title"

		exp := c.Substring(c.Field("system.title'DELETE FROM work_items"), c.Literal(title))
		where, _, _, compileErrors := workitem.Compile(exp)
		require.NotEmpty(t, compileErrors)
		assert.Len(t, compileErrors, 1)
		assert.Equal(t, "", where)
	})
}
