package sdt

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type Test struct {
	Name     string                 `json:"name" yaml:"name"`
	Input    map[string]interface{} `json:"input" yaml:"input"`
	Errors   []string               `json:"errors" yaml:"errors"`
	Expected interface{}            `json:"expected" yaml:"expected"`
}

func (test *Test) ErrorsFail(t *testing.T, errs []error) bool {
	if len(errs) > 0 {
		if len(test.Errors) > 0 {
			// We expected errors. Match them!
		outer:
			for _, expected := range test.Errors {
				for _, actual := range errs {
					if strings.Contains(actual.Error(), expected) {
						continue outer
					}
				}
				t.Error(fmt.Errorf("expected '%s' but found %v", expected, errs))
			}

			return true
		}

		t.Error(errs)
		return true
	}

	return false
}

type Fixture struct {
	Document Document `json:"document" yaml:"document"`
	Tests    []Test   `json:"tests" yaml:"tests"`
}

func TestFixtures(t *testing.T) {
	files, err := os.ReadDir("fixtures")
	if err != nil {
		t.Error(err)
	}

	for _, file := range files {
		filename, _ := filepath.Abs(path.Join("fixtures", file.Name()))
		if strings.HasSuffix(filename, ".yaml") {
			b, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Error(err)
				return
			}

			var f *Fixture
			if err := yaml.Unmarshal(b, &f); err != nil {
				t.Error(err)
				return
			}
			f.Document.Filename = filename + "#/document"

			for i, test := range f.Tests {
				t.Run(fmt.Sprintf("%s-%d-%s", file.Name(), i, test.Name), func(t *testing.T) {
					if test.ErrorsFail(t, f.Document.ValidateTemplate()) {
						return
					}

					err := f.Document.ValidateInput(test.Input)
					if err != nil && test.ErrorsFail(t, []error{err}) {
						return
					}

					out, errs := f.Document.Render(test.Input)
					if test.ErrorsFail(t, errs) {
						return
					}

					err = f.Document.ValidateOutput(out)
					if err != nil && test.ErrorsFail(t, []error{err}) {
						return
					}

					assert.Equal(t, test.Expected, out)
				})
			}
		}
	}
}
