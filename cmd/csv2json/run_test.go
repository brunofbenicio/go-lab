package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func runCLI(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestConversions(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name: "pretty output by default", stdin: "name,age\nAda,36\nLinus,54\n",
			want: "[\n  {\n    \"name\": \"Ada\",\n    \"age\": \"36\"\n  },\n  {\n    \"name\": \"Linus\",\n    \"age\": \"54\"\n  }\n]\n",
		},
		{"compact output", []string{"-compact"}, "name,age\nAda,36\n", `[{"name":"Ada","age":"36"}]` + "\n"},
		{
			"typed values", []string{"-compact", "-types"},
			"n,f,b,empty,zip,sci,word\n42,-1.5,true,,007,1e3,NaN\n",
			`[{"n":42,"f":-1.5,"b":true,"empty":null,"zip":"007","sci":1e3,"word":"NaN"}]` + "\n",
		},
		{"untyped values stay strings", []string{"-compact"}, "n,b\n42,true\n", `[{"n":"42","b":"true"}]` + "\n"},
		{"semicolon delimiter", []string{"-compact", "-d", ";"}, "a;b\n1;2\n", `[{"a":"1","b":"2"}]` + "\n"},
		{"tab delimiter written as \\t", []string{"-compact", "-d", `\t`}, "a\tb\n1\t2\n", `[{"a":"1","b":"2"}]` + "\n"},
		{"tab delimiter as a real tab", []string{"-compact", "-d", "\t"}, "a\tb\n1\t2\n", `[{"a":"1","b":"2"}]` + "\n"},
		{
			"no header row", []string{"-compact", "-no-header"}, "a,b\nc,d\n",
			`[{"col1":"a","col2":"b"},{"col1":"c","col2":"d"}]` + "\n",
		},
		{
			"quoted fields", []string{"-compact"}, "name,quote\n\"Doe, J.\",\"say \"\"hi\"\"\nthere\"\n",
			`[{"name":"Doe, J.","quote":"say \"hi\"\nthere"}]` + "\n",
		},
		{"html characters are not escaped", []string{"-compact"}, "a\n<b>&</b>\n", `[{"a":"<b>&</b>"}]` + "\n"},
		{"column order is preserved", []string{"-compact"}, "z,a,m\n1,2,3\n", `[{"z":"1","a":"2","m":"3"}]` + "\n"},
		{"unicode", []string{"-compact"}, "nome\nção\n", `[{"nome":"ção"}]` + "\n"},
		{"byte order mark is dropped", []string{"-compact"}, "\xef\xbb\xbfid\n1\n", `[{"id":"1"}]` + "\n"},
		{"crlf line endings", []string{"-compact"}, "a,b\r\n1,2\r\n", `[{"a":"1","b":"2"}]` + "\n"},
		{"header only", nil, "a,b\n", "[]\n"},
		{"empty input", nil, "", "[]\n"},
		{"explicit dash reads stdin", []string{"-compact", "-"}, "a\n1\n", `[{"a":"1"}]` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.stdin, tt.args...)
			if code != 0 || stdout != tt.want || stderr != "" {
				t.Errorf("run(%q) = (%d, %q, %q), want (0, %q, \"\")", tt.args, code, stdout, stderr, tt.want)
			}
			if !json.Valid([]byte(stdout)) {
				t.Errorf("output is not valid JSON: %q", stdout)
			}
		})
	}
}

func TestFileOperand(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("people.csv", []byte("name\nAda\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI("ignored", "-compact", "people.csv")
	if want := `[{"name":"Ada"}]` + "\n"; code != 0 || stdout != want || stderr != "" {
		t.Errorf("run() = (%d, %q, %q), want (0, %q, \"\")", code, stdout, stderr, want)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantErrHas string
	}{
		{"duplicate column", nil, "a,a\n1,2\n", 1, "duplicate column name"},
		{"empty column name", nil, "a,,c\n1,2,3\n", 1, "empty name"},
		{"row with too few fields", nil, "a,b\n1\n", 1, "wrong number of fields"},
		{"row with too many fields", nil, "a\n1,2\n", 1, "wrong number of fields"},
		{"unterminated quote", nil, "a\n\"oops\n", 1, "extraneous or missing"},
		{"missing file", []string{"nope.csv"}, "", 1, "no such file"},
		{"two files", []string{"a.csv", "b.csv"}, "", 2, "at most one file"},
		{"empty delimiter", []string{"-d", ""}, "", 2, "-d must be a single character"},
		{"long delimiter", []string{"-d", "ab"}, "", 2, "-d must be a single character"},
		{"quote as delimiter", []string{"-d", `"`}, "", 2, "-d cannot be"},
		{"unknown flag", []string{"-z"}, "", 2, "not defined"},
		{"help", []string{"-h"}, "", 0, "usage: csv2json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.stdin, tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want code %d, empty stdout, stderr containing %q",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantErrHas)
			}
		})
	}
}
