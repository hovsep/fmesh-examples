package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportOnSampleLog(t *testing.T) {
	log, err := os.Open("access.log")
	require.NoError(t, err)
	defer log.Close()

	fm, err := getMesh(log)
	require.NoError(t, err)
	require.NoError(t, fm.ComponentByName("read-stdin").InputByName(portStart).PutSignals(signal.New("go")))

	_, err = fm.Run(context.Background())
	require.NoError(t, err)

	// The sections come out in the order of the report's port names, not in
	// the order the branches finished: requests is done first, errors last.
	assert.Equal(t, `Requests: 17
Top clients:
    6  10.0.0.7
    5  10.0.0.9
    3  10.0.0.3
Failing paths:
    4  /api/orders
    2  /api/cart
    1  /api/search
`, report(fm))
}

func TestFailingCommandReportsStderr(t *testing.T) {
	_, err := run(context.Background(), []string{"sort", "--no-such-flag"}, strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sort --no-such-flag")
}
