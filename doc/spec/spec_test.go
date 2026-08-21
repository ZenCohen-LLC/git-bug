// Package spec executes the conformance vectors that accompany the data-format
// specification in this directory.
//
// The vectors live in testdata/*.json and, until now, were never run: the
// fixtures instructed the reader to "run 'go test ./doc/spec/... -run TestVectors"
// but no such test existed, so two expected_id values were left as placeholders.
//
// What these tests prove, and what they do not:
//
//   - Derivations are computed with entity.DeriveId, the production function, so a
//     change to how git-bug hashes is caught here.
//   - Expected values are frozen in testdata. Regenerate deliberately with
//     `go test ./doc/spec/... -Update` and review the diff, exactly as with the
//     golden files under commands/.
//   - Two anchors are externally verifiable and therefore not self-referential: the
//     SHA-256 of the empty input, and git's empty-blob SHA-1.
//
// This is drift detection over the documented derivations. It is NOT full
// conformance: proving the format would require an independent decoder reading git
// objects from the spec text and agreeing with the Go implementation. The
// operationPack struct is unexported, so no test outside package dag can marshal
// one; these vectors therefore pin the documented rule rather than the struct's
// exact wire bytes.
package spec

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/git-bug/git-bug/entity"
)

// Update rewrites the placeholder expected_id values in testdata from the current
// implementation. Named to match the -Update convention used by commands/cmdtest.
var Update = flag.Bool("Update", false, "Update expected values in testdata")

// placeholderPrefix marks a value the fixture author left to be generated.
const placeholderPrefix = "<"

// gitEmptyBlob is the SHA-1 git assigns to a zero-length blob. Every metadata
// entry in an entity tree points at it.
const gitEmptyBlob = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"

type idVector struct {
	Description  string          `json:"description"`
	InputHex     *string         `json:"input_hex"`
	Input        json.RawMessage `json:"input"`
	FirstVersion json.RawMessage `json:"first_version_input"`
	ExpectedId   string          `json:"expected_id"`
}

// rawInput returns whichever input field this vector carries.
func (v idVector) rawInput() json.RawMessage {
	if len(v.Input) > 0 {
		return v.Input
	}
	return v.FirstVersion
}

type entry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Hash string `json:"hash"`
}

type treeVector struct {
	Description string  `json:"description"`
	Entries     []entry `json:"entries"`
}

// example covers the three shapes the fixtures use for illustrative payloads:
// dag-entity.json and identity.json nest them under "json", while bug.json uses
// "ops_blob" and carries its tree entries inline.
type example struct {
	Description string          `json:"description"`
	JSON        json.RawMessage `json:"json"`
	OpsBlob     json.RawMessage `json:"ops_blob"`
	TreeEntries []entry         `json:"tree_entries"`
}

// payload returns whichever body field this example carries.
func (e example) payload() json.RawMessage {
	if len(e.JSON) > 0 {
		return e.JSON
	}
	return e.OpsBlob
}

type fixture struct {
	IdDerivation    []idVector   `json:"id_derivation"`
	TreeEntries     []treeVector `json:"tree_entries"`
	PackExamples    []example    `json:"operation_pack_examples"`
	VersionExamples []example    `json:"version_examples"`
}

func fixturePath(name string) string { return filepath.Join("testdata", name) }

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	data, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	var f fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return f
}

// compact strips insignificant whitespace while preserving key order, which is
// what makes the hash reproducible: Go marshals struct fields in declaration
// order, not alphabetically, and the fixtures record that order.
func compact(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compacting: %v", err)
	}
	return buf.Bytes()
}

// TestDeriveIdAnchors checks entity.DeriveId against a value that is true
// independently of git-bug: the SHA-256 of the empty input. If this fails, the
// hash function itself changed, and every ID in every repository changed with it.
func TestDeriveIdAnchors(t *testing.T) {
	const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := entity.DeriveId([]byte{}); got.String() != emptySHA256 {
		t.Errorf("DeriveId(empty) = %s, want %s", got, emptySHA256)
	}
}

// TestGitEmptyBlobHash checks the constant the fixtures use for every metadata
// tree entry really is git's empty blob, per git's object format
// ("blob <len>\x00<content>" hashed with SHA-1).
func TestGitEmptyBlobHash(t *testing.T) {
	sum := sha1.Sum([]byte("blob 0\x00"))
	if got := fmt.Sprintf("%x", sum); got != gitEmptyBlob {
		t.Errorf("git empty blob = %s, want %s", got, gitEmptyBlob)
	}
}

// TestVectors runs the id_derivation vectors in every fixture. Run with -Update
// to fill in placeholders, then review the diff before committing.
func TestVectors(t *testing.T) {
	for _, name := range []string{"dag-entity.json", "identity.json", "bug.json"} {
		t.Run(name, func(t *testing.T) {
			f := loadFixture(t, name)
			if len(f.IdDerivation) == 0 {
				t.Skip("no id_derivation vectors")
			}
			for _, v := range f.IdDerivation {
				t.Run(v.Description, func(t *testing.T) {
					var got entity.Id
					switch {
					case v.InputHex != nil:
						raw, err := decodeHex(*v.InputHex)
						if err != nil {
							t.Fatalf("bad input_hex: %v", err)
						}
						got = entity.DeriveId(raw)
					case len(v.rawInput()) > 0:
						got = entity.DeriveId(compact(t, v.rawInput()))
					default:
						// bug.json states its rule in prose and points at the
						// ops_blob examples instead of carrying its own input.
						// TestEntityIdIsFirstOperation covers that rule.
						t.Skip("vector carries no input of its own")
					}

					if strings.HasPrefix(v.ExpectedId, placeholderPrefix) {
						if *Update {
							updatePlaceholder(t, name, v.ExpectedId, got.String())
							t.Logf("filled placeholder with %s", got)
							return
						}
						t.Fatalf("expected_id is still a placeholder (%q); "+
							"run: go test ./doc/spec/... -Update", v.ExpectedId)
					}
					if got.String() != v.ExpectedId {
						t.Errorf("derived %s, fixture says %s", got, v.ExpectedId)
					}
				})
			}
		})
	}
}

// TestEntityIdIsFirstOperation pins the distinction the dag-entity fixture does
// not draw. An entity's ID is the hash of its FIRST OPERATION, not of the
// operationPack that carries it (doc/design/data-model.md, "Entities and
// Operation's ID"). The two differ, and confusing them yields an ID that matches
// no ref on disk.
func TestEntityIdIsFirstOperation(t *testing.T) {
	checked := 0
	for _, name := range []string{"dag-entity.json", "bug.json"} {
		f := loadFixture(t, name)

		var packs []json.RawMessage
		for _, v := range f.IdDerivation {
			if raw := v.rawInput(); len(raw) > 0 {
				packs = append(packs, raw)
			}
		}
		for _, ex := range f.PackExamples {
			if raw := ex.payload(); len(raw) > 0 {
				packs = append(packs, raw)
			}
		}

		for i, raw := range packs {
			var pack struct {
				Ops []json.RawMessage `json:"ops"`
			}
			if err := json.Unmarshal(raw, &pack); err != nil || len(pack.Ops) == 0 {
				continue // merge packs carry no operations
			}
			t.Run(fmt.Sprintf("%s/pack-%d", name, i), func(t *testing.T) {
				packId := entity.DeriveId(compact(t, raw))
				entityId := entity.DeriveId(compact(t, pack.Ops[0]))
				if packId == entityId {
					t.Fatal("pack ID and entity ID coincide; this vector can no " +
						"longer tell the two derivations apart")
				}
				t.Logf("pack ID   = hex(sha256(ops blob))  = %s", packId)
				t.Logf("entity ID = hex(sha256(ops[0]))    = %s", entityId)
			})
			checked++
		}
	}
	if checked == 0 {
		t.Skip("no vector carries an operation pack")
	}
}

// TestTreeEntriesSorted checks git's ordering requirement: tree entries are
// listed lexicographically by name.
func TestTreeEntriesSorted(t *testing.T) {
	forEachTree(t, func(t *testing.T, tv treeVector) {
		names := make([]string, len(tv.Entries))
		for i, e := range tv.Entries {
			names[i] = e.Name
		}
		if !sort.StringsAreSorted(names) {
			t.Errorf("entries not lexicographically sorted: %v", names)
		}
	})
}

// TestTreeEntryStructure checks the invariants the spec states for an entity
// tree: exactly one ops blob, at most one create-clock (root commits only), and
// empty-blob hashes on every metadata entry.
func TestTreeEntryStructure(t *testing.T) {
	forEachTree(t, func(t *testing.T, tv treeVector) {
		counts := map[string]int{}
		for _, e := range tv.Entries {
			switch {
			case e.Name == "ops" || e.Name == "version":
				counts["payload"]++
			case strings.HasPrefix(e.Name, "create-clock-"):
				counts["create"]++
			case strings.HasPrefix(e.Name, "edit-clock-"):
				counts["edit"]++
			case strings.HasPrefix(e.Name, "version-"):
				counts["formatVersion"]++
			}
			// Metadata entries carry no content, so they must be the empty blob.
			isMetadata := strings.HasPrefix(e.Name, "create-clock-") ||
				strings.HasPrefix(e.Name, "edit-clock-") ||
				strings.HasPrefix(e.Name, "version-")
			if isMetadata && e.Hash != gitEmptyBlob {
				t.Errorf("metadata entry %q has hash %s, want the empty blob %s",
					e.Name, e.Hash, gitEmptyBlob)
			}
		}
		if counts["payload"] != 1 {
			t.Errorf("want exactly one ops/version blob, got %d", counts["payload"])
		}
		if counts["create"] > 1 {
			t.Errorf("want at most one create-clock entry, got %d", counts["create"])
		}
	})
}

// TestExamplesWellFormed checks the illustrative payloads actually satisfy the
// field requirements the spec states for them.
func TestExamplesWellFormed(t *testing.T) {
	for _, name := range []string{"dag-entity.json", "identity.json", "bug.json"} {
		f := loadFixture(t, name)
		examples := make([]example, 0, len(f.PackExamples)+len(f.VersionExamples))
		examples = append(examples, f.PackExamples...)
		examples = append(examples, f.VersionExamples...)
		for _, ex := range examples {
			t.Run(name+"/"+ex.Description, func(t *testing.T) {
				var probe map[string]json.RawMessage
				if err := json.Unmarshal(ex.payload(), &probe); err != nil {
					t.Fatalf("example is not a JSON object: %v", err)
				}
				if ops, isPack := probe["ops"]; isPack {
					if _, hasAuthor := probe["author"]; !hasAuthor {
						t.Error("operation pack has no author")
					}
					var list []map[string]json.RawMessage
					if err := json.Unmarshal(ops, &list); err != nil {
						t.Fatalf("ops is not an array: %v", err)
					}
					for i, op := range list {
						for _, field := range []string{"type", "timestamp", "nonce"} {
							if _, ok := op[field]; !ok {
								t.Errorf("ops[%d] missing required field %q", i, field)
							}
						}
					}
				}
			})
		}
	}
}

func forEachTree(t *testing.T, check func(*testing.T, treeVector)) {
	t.Helper()
	for _, name := range []string{"dag-entity.json", "identity.json", "bug.json"} {
		f := loadFixture(t, name)
		trees := append([]treeVector(nil), f.TreeEntries...)
		// bug.json attaches its tree entries to each operation_pack example
		// rather than to a top-level tree_entries block.
		for _, ex := range f.PackExamples {
			if len(ex.TreeEntries) > 0 {
				trees = append(trees, treeVector{
					Description: ex.Description,
					Entries:     ex.TreeEntries,
				})
			}
		}
		for _, tv := range trees {
			t.Run(name+"/"+tv.Description, func(t *testing.T) { check(t, tv) })
		}
	}
}

func decodeHex(s string) ([]byte, error) {
	if s == "" {
		return []byte{}, nil
	}
	out := make([]byte, 0, len(s)/2)
	var hi byte
	for i := 0; i < len(s); i++ {
		var nib byte
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			nib = c - '0'
		case c >= 'a' && c <= 'f':
			nib = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			nib = c - 'A' + 10
		default:
			return nil, fmt.Errorf("invalid hex character %q", c)
		}
		if i%2 == 0 {
			hi = nib
		} else {
			out = append(out, hi<<4|nib)
		}
	}
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd-length hex string")
	}
	return out, nil
}

// updatePlaceholder rewrites one placeholder in place. It edits the raw bytes
// rather than re-marshalling so the fixture keeps its comments and layout.
func updatePlaceholder(t *testing.T, name, placeholder, value string) {
	t.Helper()
	path := fixturePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	quoted := `"` + placeholder + `"`
	if !bytes.Contains(data, []byte(quoted)) {
		t.Fatalf("placeholder %s not found verbatim in %s", quoted, name)
	}
	data = bytes.Replace(data, []byte(quoted), []byte(`"`+value+`"`), 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
