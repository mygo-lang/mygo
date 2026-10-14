package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapTransitiveReturnTypes(t *testing.T) {
	for _, scenario := range []struct{ name, extraImport, binding, alias, otherAlias, conversion string }{
		{"inferred", "", "let identity", "auth", "auth2", "identity.Name"},
		{"alternate-alias", "import identitypkg \"example.com/bootstrap-test/auth\"", "let identity", "identitypkg", "auth", "identity.Name"},
		{"alias-conflict", "import auth \"go:fmt\"", "let identity", "auth2", "auth3", "auth.Sprint(identity.Name)"},
		{"annotated-workaround", "import identitypkg \"example.com/bootstrap-test/auth\"", "let identity: identitypkg.Identity", "identitypkg", "auth", "identity.Name"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			bootstrapTestModule(t, root)
			sources := map[string]string{
				"auth/auth.mygo": `package auth
struct Identity
  Name: String
end
`,
				"other/auth/auth.mygo": "package auth\nstruct Identity\n ID: Int\nend\n",
				"altstorage/storage.mygo": `package altstorage
import auth "example.com/bootstrap-test/other/auth"
func Create() -> Result[auth.Identity, String]
  Ok(auth.Identity { ID: 9 })
end
`,
				"storage/storage.mygo": `package storage
import auth "example.com/bootstrap-test/auth"
struct Identity
  Local: Bool
end
struct AccountRepository
  Name: String
end
impl AccountRepository
  func Create(self: AccountRepository) -> Result[auth.Identity, String]
    Ok(auth.Identity { Name: self.Name })
  end
end
func Create(name: String) -> Result[auth.Identity, String]
  Ok(auth.Identity { Name: name })
end
func Nested() -> Result[Slice[auth.Identity], String]
  Ok([auth.Identity { Name: "nested" }])
end
func Pair() -> Result[(auth.Identity, Int), String]
  Ok((auth.Identity { Name: "tuple" }, 7))
end
func Factory() -> func(String) -> auth.Identity
  func(name: String) -> auth.Identity
    auth.Identity { Name: name }
  end
end
`,
				"server/server.mygo": `package server
import storage "example.com/bootstrap-test/storage"
import altstorage "example.com/bootstrap-test/altstorage"
EXTRA_IMPORT
func Name() -> String
  let repository = storage.AccountRepository { Name: "method" }
  BINDING = repository.Create().Unwrap()
  let nested = storage.Nested().Unwrap().Get(0).Unwrap()
  let (pair, count) = storage.Pair().Unwrap()
  var factory = storage.Factory()
  CONVERSION + storage.Create("function").Unwrap().Name + nested.Name + pair.Name + factory("callback").Name + count.ToString() + altstorage.Create().Unwrap().ID.ToString()
end
`,
				"server/unused.mygo": "package server\nfunc Number() -> Int\n  7\nend\n",
				"server/server_test.go": `package server
import "testing"
func TestName(t *testing.T) {
  if got := Name(); got != "methodfunctionnestedtuplecallback79" { t.Fatal(got) }
}
`,
			}
			sources["server/server.mygo"] = strings.NewReplacer("EXTRA_IMPORT", scenario.extraImport, "BINDING", scenario.binding, "CONVERSION", scenario.conversion).Replace(sources["server/server.mygo"])
			for name, source := range sources {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := CompileDirBootstrap(filepath.Join(root, "server")); err != nil {
				t.Fatal(err)
			}
			generated, err := os.ReadFile(filepath.Join(root, "server", "zz_server.gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`"example.com/bootstrap-test/auth"`, `"example.com/bootstrap-test/other/auth"`, "Unwrap[" + scenario.alias + ".Identity, string]", "Unwrap[" + scenario.otherAlias + ".Identity, string]", "Unwrap[[]" + scenario.alias + ".Identity, string]", "var pair " + scenario.alias + ".Identity"} {
				if !strings.Contains(string(generated), want) {
					t.Fatalf("missing %q:\n%s", want, generated)
				}
			}
			unused, err := os.ReadFile(filepath.Join(root, "server", "zz_unused.gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(unused), "bootstrap-test/auth") || strings.Contains(string(unused), "bootstrap-test/other/auth") {
				t.Fatalf("unused inferred import:\n%s", unused)
			}
			if strings.Count(string(generated), `"example.com/bootstrap-test/auth"`) != 1 {
				t.Fatalf("duplicate import:\n%s", generated)
			}
			if _, err := CompileDirBootstrap(filepath.Join(root, "server")); err != nil {
				t.Fatal(err)
			}
			repeated, err := os.ReadFile(filepath.Join(root, "server", "zz_server.gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			if string(repeated) != string(generated) {
				t.Fatal("repeat generation changed imports or type arguments")
			}
			cmd := exec.Command("go", "test", "-buildvcs=false", "./...")
			cmd.Dir = root
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated module: %v\n%s", err, output)
			}
		})
	}
}
