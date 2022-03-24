package wireless

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func parseNetwork(b []byte) ([]Network, error) {
	i := bytes.Index(b, []byte("\n"))
	if i > 0 {
		b = b[i:]
	}

	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = '\t'
	r.FieldsPerRecord = 4

	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	nts := []Network{}
	for _, rec := range recs {
		id, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, errors.Wrap(err, "parse id")
		}

		nts = append(nts, Network{
			ID:    id,
			SSID:  rec[1],
			BSSID: rec[2],
			Flags: parseFlags(rec[3]),
		})
	}

	return nts, nil
}

func parseFlags(s string) []string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	flags := strings.Split(s, "][")
	if len(flags) == 1 && flags[0] == "" {
		return []string{}
	}

	return flags
}

func parseAP(b []byte) ([]AP, error) {
	i := bytes.Index(b, []byte("\n"))
	if i > 0 {
		b = b[i:]
	}

	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = '\t'
	r.FieldsPerRecord = 5

	aps := []AP{}

	for rec, err := r.Read(); err != io.EOF; rec, err = r.Read() {
		if err == csv.ErrFieldCount {
			// Skip this record, as it's probably malformed.
			continue
		}
		if err != nil {
			return nil, err
		}
		if rec == nil {
			continue
		}
		bssid, err := net.ParseMAC(rec[0])
		if err != nil {
			continue
		}

		fr, err := strconv.Atoi(rec[1])
		if err != nil {
			continue
		}

		ss, err := strconv.Atoi(rec[2])
		if err != nil {
			continue
		}

		aps = append(aps, AP{
			BSSID:     bssid,
			SSID:      rec[4],
			Frequency: fr,
			Signal:    ss,
			Flags:     parseFlags(rec[3]),
		})
	}

	return aps, nil
}

func quote(s string) string {
	return fmt.Sprintf("\"%s\"", s)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func unquote(s string) string {
	return strings.Trim(s, `"`)
}
