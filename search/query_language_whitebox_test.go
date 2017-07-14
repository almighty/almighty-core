package search

import (
	"encoding/json"
	"runtime/debug"
	"testing"

	c "github.com/fabric8-services/fabric8-wit/criteria"
	"github.com/fabric8-services/fabric8-wit/resource"
	w "github.com/fabric8-services/fabric8-wit/workitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMap(t *testing.T) {
	resource.Require(t, resource.UnitTest)
	t.Parallel()

	t.Run(Q_AND, func(t *testing.T) {
		t.Parallel()
		// given
		input := `{"` + Q_AND + `": [{"space": "openshiftio"}, {"status": "NEW"}]}`
		// Parsing/Unmarshalling JSON encoding/json
		fm := map[string]interface{}{}
		err := json.Unmarshal([]byte(input), &fm)
		if err != nil {
			panic(err)
		}
		// when
		actualQuery := Query{}
		parseMap(fm, &actualQuery)
		// then
		openshiftio := "openshiftio"
		status := "NEW"
		expectedQuery := Query{Name: Q_AND, Children: []Query{
			{Name: "space", Value: &openshiftio},
			{Name: "status", Value: &status}},
		}
		assert.Equal(t, expectedQuery, actualQuery)
	})

	t.Run("Minimal OR and AND operation", func(t *testing.T) {
		t.Parallel()
		input := `
			{"` + Q_OR + `": [{"` + Q_AND + `": [{"space": "openshiftio"},
                         {"area": "planner"}]},
	        {"` + Q_AND + `": [{"space": "rhel"}]}]}`
		fm := map[string]interface{}{}

		// Parsing/Unmarshalling JSON encoding/json
		err := json.Unmarshal([]byte(input), &fm)

		if err != nil {
			panic(err)
		}
		q := &Query{}

		parseMap(fm, q)

		openshiftio := "openshiftio"
		area := "planner"
		rhel := "rhel"
		expected := &Query{Name: Q_OR, Children: []Query{
			{Name: Q_AND, Children: []Query{
				{Name: "space", Value: &openshiftio},
				{Name: "area", Value: &area}}},
			{Name: Q_AND, Children: []Query{
				{Name: "space", Value: &rhel}}},
		}}
		assert.Equal(t, expected, q)
	})

	t.Run("minimal OR and AND and Negate operation", func(t *testing.T) {
		t.Parallel()
		input := `
		{"` + Q_OR + `": [{"` + Q_AND + `": [{"space": "openshiftio"},
                         {"area": "planner"}]},
			 {"` + Q_AND + `": [{"space": "rhel", "negate": true}]}]}`
		fm := map[string]interface{}{}

		// Parsing/Unmarshalling JSON encoding/json
		err := json.Unmarshal([]byte(input), &fm)

		if err != nil {
			panic(err)
		}
		q := &Query{}

		parseMap(fm, q)

		openshiftio := "openshiftio"
		area := "planner"
		rhel := "rhel"
		expected := &Query{Name: Q_OR, Children: []Query{
			{Name: Q_AND, Children: []Query{
				{Name: "space", Value: &openshiftio},
				{Name: "area", Value: &area}}},
			{Name: Q_AND, Children: []Query{
				{Name: "space", Value: &rhel, Negate: true}}},
		}}
		assert.Equal(t, expected, q)
	})
}
func TestGenerateExpression(t *testing.T) {
	resource.Require(t, resource.UnitTest)
	t.Parallel()

	t.Run("Equals (top-level)", func(t *testing.T) {
		t.Parallel()
		// given
		spaceName := "openshiftio"
		q := Query{Name: "space", Value: &spaceName}
		// when
		actualExpr := q.generateExpression()
		// then
		expectedExpr := c.Equals(
			c.Field("SpaceID"),
			c.Literal(spaceName),
		)
		expectEqualExpr(t, expectedExpr, actualExpr)
	})

	t.Run(Q_NOT+" (top-level)", func(t *testing.T) {
		t.Parallel()
		// given
		spaceName := "openshiftio"
		q := Query{Name: "space", Value: &spaceName, Negate: true}
		// when
		actualExpr := q.generateExpression()
		// then
		expectedExpr := c.Not(
			c.Field("SpaceID"),
			c.Literal(spaceName),
		)
		expectEqualExpr(t, expectedExpr, actualExpr)
	})

	t.Run(Q_AND, func(t *testing.T) {
		t.Parallel()
		// given
		statusName := "NEW"
		spaceName := "openshiftio"
		q := Query{
			Name: Q_AND,
			Children: []Query{
				{Name: "space", Value: &spaceName},
				{Name: "status", Value: &statusName},
			},
		}
		// when
		actualExpr := q.generateExpression()
		// then
		expectedExpr := c.And(
			c.Equals(
				c.Field("SpaceID"),
				c.Literal(spaceName),
			),
			c.Equals(
				c.Field("status"),
				c.Literal(statusName),
			),
		)
		expectEqualExpr(t, expectedExpr, actualExpr)
	})

	t.Run(Q_OR, func(t *testing.T) {
		t.Parallel()
		// given
		statusName := "NEW"
		spaceName := "openshiftio"
		q := Query{
			Name: Q_OR,
			Children: []Query{
				{Name: "space", Value: &spaceName},
				{Name: "status", Value: &statusName},
			},
		}
		// when
		actualExpr := q.generateExpression()
		// then
		expectedExpr := c.Or(
			c.Equals(
				c.Field("SpaceID"),
				c.Literal(spaceName),
			),
			c.Equals(
				c.Field("status"),
				c.Literal(statusName),
			),
		)
		expectEqualExpr(t, expectedExpr, actualExpr)
	})

	t.Run(Q_NOT+" (nested)", func(t *testing.T) {
		t.Parallel()
		// given
		statusName := "NEW"
		spaceName := "openshiftio"
		q := Query{
			Name: Q_AND,
			Children: []Query{
				{Name: "space", Value: &spaceName, Negate: true},
				{Name: "status", Value: &statusName},
			},
		}
		// when
		actualExpr := q.generateExpression()
		// then
		expectedExpr := c.And(
			c.Not(
				c.Field("SpaceID"),
				c.Literal(spaceName),
			),
			c.Equals(
				c.Field("status"),
				c.Literal(statusName),
			),
		)
		expectEqualExpr(t, expectedExpr, actualExpr)
	})
}

func expectEqualExpr(t *testing.T, expectedExpr, actualExpr c.Expression) {
	actualClause, actualParameters, actualErrs := w.Compile(actualExpr)
	if len(actualErrs) > 0 {
		debug.PrintStack()
		require.Nil(t, actualErrs, "failed to compile actual expression")
	}
	exprectedClause, expectedParameters, expectedErrs := w.Compile(expectedExpr)
	if len(expectedErrs) > 0 {
		debug.PrintStack()
		require.Nil(t, expectedErrs, "failed to compile expected expression")
	}
	require.Equal(t, exprectedClause, actualClause, "where clause differs")
	require.Equal(t, expectedParameters, actualParameters, "parameters differ")
}
