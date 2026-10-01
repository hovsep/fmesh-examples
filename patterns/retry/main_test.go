package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemporaryErrorsAreRetried(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	body, attempts, err := fetch(fm, "report.csv")
	require.NoError(t, err)
	assert.Equal(t, "contents of report.csv", body)
	assert.Equal(t, flakyFailures+1, attempts)
}

func TestPermanentErrorIsNotRetried(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	_, attempts, err := fetch(fm, "missing.csv")
	require.ErrorIs(t, err, errNotFound)
	assert.Equal(t, 1, attempts)
}
