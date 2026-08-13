package reporter

import (
	"encoding/json"
	"fmt"
	"io"

	"svcprobe/pkg/probe"
)

// PrintJSON formats and outputs the suite result as JSON.
func PrintJSON(w io.Writer, suite probe.SuiteResult, pretty bool) error {
	var data []byte
	var err error

	if pretty {
		data, err = json.MarshalIndent(suite, "", "  ")
	} else {
		data, err = json.Marshal(suite)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON suite result: %w", err)
	}

	_, err = fmt.Fprintln(w, string(data))
	return err
}
