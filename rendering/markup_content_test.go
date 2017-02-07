package rendering

import "testing"
import "github.com/stretchr/testify/assert"
import "github.com/stretchr/testify/require"

func TestGetDefaultMarkupFromNil(t *testing.T) {
	// when
	result := NilSafeGetMarkup(nil)
	// then
	require.NotNil(t, result)
	assert.Equal(t, SystemMarkupDefault, result)
}

func TestGetMarkupFromValue(t *testing.T) {
	// given
	markup := SystemMarkupMarkdown
	// when
	result := NilSafeGetMarkup(&markup)
	// then
	require.NotNil(t, result)
	assert.Equal(t, markup, result)
}

func TestGetMarkupFromEmptyValue(t *testing.T) {
	// given
	markup := ""
	// when
	result := NilSafeGetMarkup(&markup)
	// then
	require.NotNil(t, result)
	assert.Equal(t, SystemMarkupDefault, result)
}
