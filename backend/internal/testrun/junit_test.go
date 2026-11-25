package testrun_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/testrun"
)

func TestParseJUnitXML(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="auth">
    <testcase name="login_ok" classname="auth" />
    <testcase name="login_bad_password" classname="auth">
      <failure message="expected 401 got 500">stacktrace</failure>
    </testcase>
    <testcase name="oauth_login" classname="auth">
      <skipped message="not enabled" />
    </testcase>
  </testsuite>
</testsuites>`

	res, err := testrun.ParseJUnitXML([]byte(raw))
	require.NoError(t, err)
	require.Len(t, res, 3)
	require.Equal(t, "passed", res[0].Status)
	require.Equal(t, "failed", res[1].Status)
	require.Equal(t, "expected 401 got 500", res[1].Comment)
	require.Equal(t, "skipped", res[2].Status)
}
