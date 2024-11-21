package cli

import (
	"fmt"
	"os"
	"strings"
)

// ValueSource is a source which can be used to look up a value,
// typically for use with a cli.Flag
type ValueSource interface {
	fmt.Stringer
	fmt.GoStringer

	// Lookup returns the value from the source and if it was found
	// or returns an empty string and false
	Lookup() (string, bool)
}

// EnvValueSource is to specifically detect env sources when
// printing help text
type EnvValueSource interface {
	IsFromEnv() bool
	Key() string
}

// ValueSourceChain contains an ordered series of ValueSource that
// allows for lookup where the first ValueSource to resolve is
// returned
type ValueSourceChain struct {
	Chain []ValueSource
}

func NewValueSourceChain(src ...ValueSource) ValueSourceChain {
	return ValueSourceChain{
		Chain: src,
	}
}

func (vsc *ValueSourceChain) Append(other ValueSourceChain) {
	vsc.Chain = append(vsc.Chain, other.Chain...)
}

func (vsc *ValueSourceChain) EnvKeys() []string {
	vals := []string{}

	for _, src := range vsc.Chain {
		if v, ok := src.(EnvValueSource); ok && v.IsFromEnv() {
			vals = append(vals, v.Key())
		}
	}

	return vals
}

func (vsc *ValueSourceChain) String() string {
	s := []string{}

	for _, vs := range vsc.Chain {
		s = append(s, vs.String())
	}

	return strings.Join(s, ",")
}

func (vsc *ValueSourceChain) GoString() string {
	s := []string{}

	for _, vs := range vsc.Chain {
		s = append(s, vs.GoString())
	}

	return fmt.Sprintf("&ValueSourceChain{Chain:{%[1]s}}", strings.Join(s, ","))
}

func (vsc *ValueSourceChain) Lookup() (string, bool) {
	s, _, ok := vsc.LookupWithSource()
	return s, ok
}

func (vsc *ValueSourceChain) LookupWithSource() (string, ValueSource, bool) {
	for _, src := range vsc.Chain {
		if value, found := src.Lookup(); found {
			return value, src, true
		}
	}

	return "", nil, false
}

// envVarValueSource encapsulates a ValueSource from an environment variable
type envVarValueSource struct {
	key string
}

func (e *envVarValueSource) Lookup() (string, bool) {
	return os.LookupEnv(strings.TrimSpace(string(e.key)))
}

func (e *envVarValueSource) IsFromEnv() bool {
	return true
}

func (e *envVarValueSource) Key() string {
	return e.key
}

func (e *envVarValueSource) String() string { return fmt.Sprintf("environment variable %[1]q", e.key) }
func (e *envVarValueSource) GoString() string {
	return fmt.Sprintf("&envVarValueSource{Key:%[1]q}", e.key)
}

func EnvVar(key string) ValueSource {
	return &envVarValueSource{
		key: key,
	}
}

// EnvVars is a helper function to encapsulate a number of
// envVarValueSource together as a ValueSourceChain
func EnvVars(keys ...string) ValueSourceChain {
	vsc := ValueSourceChain{Chain: []ValueSource{}}

	for _, key := range keys {
		vsc.Chain = append(vsc.Chain, EnvVar(key))
	}

	return vsc
}

// fileValueSource encapsulates a ValueSource from a file
type fileValueSource struct {
	Path string
}

func (f *fileValueSource) Lookup() (string, bool) {
	data, err := os.ReadFile(f.Path)
	return string(data), err == nil
}

func (f *fileValueSource) String() string { return fmt.Sprintf("file %[1]q", f.Path) }
func (f *fileValueSource) GoString() string {
	return fmt.Sprintf("&fileValueSource{Path:%[1]q}", f.Path)
}

func File(path string) ValueSource {
	return &fileValueSource{Path: path}
}

// Files is a helper function to encapsulate a number of
// fileValueSource together as a ValueSourceChain
func Files(paths ...string) ValueSourceChain {
	vsc := ValueSourceChain{Chain: []ValueSource{}}

	for _, path := range paths {
		vsc.Chain = append(vsc.Chain, File(path))
	}

	return vsc
}

type MapSource struct {
	name string
	m    map[any]any
}

func NewMapSource(name string, m map[any]any) *MapSource {
	return &MapSource{
		name: name,
		m:    m,
	}
}

func (ms *MapSource) lookup(name string) (any, bool) {
	// nestedVal checks if the name has '.' delimiters.
	// If so, it tries to traverse the tree by the '.' delimited sections to find
	// a nested value for the key.
	if sections := strings.Split(name, "."); len(sections) > 1 {
		node := ms.m
		for _, section := range sections[:len(sections)-1] {
			child, ok := node[section]
			if !ok {
				return nil, false
			}

			switch child := child.(type) {
			case map[string]any:
				node = make(map[any]any, len(child))
				for k, v := range child {
					node[k] = v
				}
			case map[any]any:
				node = child
			default:
				return nil, false
			}
		}
		if val, ok := node[sections[len(sections)-1]]; ok {
			return val, true
		}
	}

	return nil, false
}

type mapValueSource struct {
	key string
	ms  *MapSource
}

func NewMapValueSource(key string, ms *MapSource) ValueSource {
	return &mapValueSource{
		key: key,
		ms:  ms,
	}
}

func (mvs *mapValueSource) String() string {
	return fmt.Sprintf("map source key %[1]q from %[2]q", mvs.key, mvs.ms.name)
}

func (mvs *mapValueSource) GoString() string {
	return fmt.Sprintf("&mapValueSource{key:%[1]q, src:%[2]q}", mvs.key, mvs.ms.m)
}

func (mvs *mapValueSource) Lookup() (string, bool) {
	if v, ok := mvs.ms.lookup(mvs.key); !ok {
		return "", false
	} else {
		return fmt.Sprintf("%+v", v), true
	}
}
