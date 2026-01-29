package errors

import "testing"

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		code Code
		want ExitCode
	}{
		{CodeCfgInvalid, ExitConfig},
		{CodeSSHDialFailed, ExitConnect},
		{CodeROBlocked, ExitReadOnly},
		{CodeDBExecFailed, ExitDBExec},
		{CodeInternal, ExitInternal},
	}
	for _, tc := range cases {
		if got := ExitCodeFor(tc.code); got != tc.want {
			t.Fatalf("ExitCodeFor(%s)=%d want %d", tc.code, got, tc.want)
		}
	}
}
