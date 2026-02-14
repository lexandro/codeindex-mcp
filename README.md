# codeindex-mcp

In-memory MCP szerver forráskód indexeléshez. A `grep` és `find` parancsok gyors, indexelt alternatívája, amelyet Claude Code (vagy bármely MCP-kompatibilis kliens) használhat.

## Mi ez?

A `codeindex-mcp` egy [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) szerver, amely induláskor memóriába indexeli a projekt összes forrásfájlját, majd 4 tool-t biztosít a kereséshez. A háttérben futó file watcher automatikusan frissíti az indexet minden fájlváltozáskor.

**Miért hasznos?**
- A `grep`/`find` parancsoknál nagyságrendekkel gyorsabb nagy kódbázisokon
- Full-text search Bleve-vel (szó, pontos kifejezés, regex)
- Glob-alapú fájlkeresés doublestar támogatással (`**/*.go`)
- Automatikus inkrementális frissítés file watcher-rel
- Konfiguálható szűrés: `.gitignore`, `.claudeignore`, egyedi minták

## Telepítés

### Előfeltételek

- Go 1.22+ ([letöltés](https://go.dev/dl/))

### Build

```bash
git clone https://github.com/lexandro/codeindex-mcp.git
cd codeindex-mcp
go build -o codeindex-mcp.exe .
```

Az eredmény egyetlen statikus bináris (~17 MB), külső dependency nélkül.

### Tesztek futtatása

```bash
go test ./...
```

## Használat

### Önálló futtatás (teszteléshez)

```bash
./codeindex-mcp.exe --root C:\projects\my-project
```

A szerver stdio-n kommunikál (stdin/stdout), tehát önállóan nem interaktív — MCP kliensből kell használni.

### Claude Code integráció

Add hozzá a Claude Code MCP beállításokhoz (`.claude/settings.json` vagy globális settings):

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "C:\\path\\to\\codeindex-mcp.exe",
      "args": ["--root", "C:\\projects\\my-project"]
    }
  }
}
```

Projekt-specifikus beállításhoz a projekt gyökerében `.mcp.json` fájlban:

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "C:\\path\\to\\codeindex-mcp.exe",
      "args": ["--root", "."]
    }
  }
}
```

Ezután a Claude Code automatikusan használhatja a `codeindex_search`, `codeindex_files`, `codeindex_status` és `codeindex_reindex` tool-okat.

## CLI paraméterek

| Paraméter | Alapértelmezés | Leírás |
|-----------|----------------|--------|
| `--root DIR` | aktuális könyvtár | A projekt gyökérkönyvtára |
| `--exclude PATTERN` | _(nincs)_ | Extra ignore minta, ismételhető (pl. `--exclude "*.generated.go" --exclude "vendor/"`) |
| `--max-file-size N` | `1048576` (1 MB) | Maximális fájlméret byte-ban; ennél nagyobb fájlok kimaradnak az indexből |
| `--max-results N` | `50` | Alapértelmezett maximum találatszám |
| `--log-level LEVEL` | `info` | Naplózási szint: `debug`, `info`, `warn`, `error` |
| `--log-file PATH` | _(stderr)_ | Napló fájl elérési útja; alapból stderr-re ír |

### Példák

```bash
# Alaphasználat - aktuális könyvtár indexelése
./codeindex-mcp.exe

# Megadott projekt gyökér, extra kizárásokkal
./codeindex-mcp.exe --root /home/user/myproject \
  --exclude "*.generated.go" \
  --exclude "testdata/"

# Debug naplózás fájlba
./codeindex-mcp.exe --root . --log-level debug --log-file /tmp/codeindex.log

# Nagyobb fájlok engedélyezése (5 MB)
./codeindex-mcp.exe --root . --max-file-size 5242880
```

## MCP Tool-ok

A szerver 4 tool-t regisztrál:

### 1. `codeindex_search` — Tartalom keresés

Full-text keresés az indexelt fájlok tartalmában.

**Paraméterek:**

| Név | Típus | Kötelező | Leírás |
|-----|-------|----------|--------|
| `query` | string | igen | Keresési kifejezés (lásd formátumok lent) |
| `fileGlob` | string | nem | Fájlszűrő glob (pl. `**/*.go`) |
| `maxResults` | int | nem | Max találat (alapértelmezett: 50) |
| `contextLines` | int | nem | Kontextus sorok a találat előtt/után (alapértelmezett: 2) |

**Query formátumok:**

| Formátum | Példa | Viselkedés |
|----------|-------|------------|
| Sima szöveg | `handleRequest` | Szó-szintű keresés (Bleve MatchQuery) |
| `"idézőjelben"` | `"func main"` | Pontos kifejezés keresés (PhraseQuery) |
| `/regex/` | `/func\s+\w+Handler/` | Reguláris kifejezés (RegexpQuery) |

**Példa kimenet:**

```
Found 3 matches in 2 files:

── main.go ──
  4: import "fmt"
  5:
  6: func main() {
  7:     fmt.Println("hello world")
  8: }

── server/server.go ──
  14: func main() {
  15:     startServer()
  16: }
```

### 2. `codeindex_files` — Fájl keresés

Glob-alapú fájlkeresés az indexelt fájlok között.

**Paraméterek:**

| Név | Típus | Kötelező | Leírás |
|-----|-------|----------|--------|
| `pattern` | string | igen | Glob minta (pl. `**/*.ts`, `src/**/*.go`) |
| `nameOnly` | bool | nem | Ha `true`, csak elérési utakat ad vissza metaadatok nélkül |
| `maxResults` | int | nem | Max találat (alapértelmezett: 50) |

**Glob minták:**

| Minta | Jelentés |
|-------|----------|
| `**/*.go` | Minden Go fájl, bármely alkönyvtárban |
| `src/**/*.ts` | TypeScript fájlok a `src/` alatt |
| `**/test_*.py` | Python teszt fájlok bárhol |
| `*.json` | JSON fájlok csak a gyökérben |

**Példa kimenet:**

```
Found 4 files:

  src/main.go  (Go, 2.1 KB, 85 lines)
  src/utils/helper.go  (Go, 1.3 KB, 42 lines)
  src/server/handler.go  (Go, 4.7 KB, 156 lines)
  src/config/config.go  (Go, 892 B, 31 lines)
```

### 3. `codeindex_status` — Index állapot

Megjeleníti az index aktuális állapotát.

**Paraméterek:** nincs

**Példa kimenet:**

```
=== codeindex-mcp Status ===

Root directory: C:\projects\my-project
Uptime: 45s
Indexed files: 1234
Content-indexed documents: 1234
Total indexed size: 8.5 MB
Memory usage: 95.2 MB (heap: 82.1 MB)

Languages:
  TypeScript           456 files
  Go                   312 files
  JavaScript           189 files
  Python               98 files
  JSON                 67 files
  YAML                 45 files
  Markdown             34 files
  Unknown              33 files
```

### 4. `codeindex_reindex` — Újraindexelés

Teljes újraindexelés — törli az indexet és újra felépíti a nulláról.

**Paraméterek:** nincs

**Példa kimenet:**

```
Reindex complete.
  Files indexed: 1234
  Total size: 8.5 MB
  Duration: 1.234s
```

## Ignore rendszer

A szerver háromrétegű szűrési rendszert használ annak eldöntésére, hogy mely fájlok kerüljenek az indexbe:

### 1. Beépített alapértelmezett minták

Automatikusan kihagyva, konfigurálás nélkül:

| Kategória | Minták |
|-----------|--------|
| Verziókezelés | `.git`, `.svn`, `.hg` |
| Függőségek | `node_modules`, `vendor`, `bower_components`, `.yarn` |
| Build kimenet | `dist`, `build`, `out`, `target`, `bin`, `obj` |
| IDE fájlok | `.idea`, `.vscode`, `.vs` |
| Binárisok | `*.exe`, `*.dll`, `*.so`, `*.dylib`, `*.class`, `*.jar` |
| Képek | `*.png`, `*.jpg`, `*.gif`, `*.webp`, `*.ico` |
| Betűtípusok | `*.woff`, `*.woff2`, `*.ttf`, `*.eot` |
| Média | `*.mp3`, `*.mp4`, `*.avi`, `*.mov` |
| Dokumentumok | `*.pdf`, `*.doc`, `*.xlsx`, `*.pptx` |
| Lock fájlok | `package-lock.json`, `yarn.lock`, `go.sum`, `Cargo.lock` |
| Tömörített | `*.zip`, `*.tar`, `*.tar.gz`, `*.rar`, `*.7z` |
| Minifikált | `*.min.js`, `*.min.css` |
| Source map | `*.map` |
| Cache | `.cache`, `.next`, `.nuxt`, `.parcel-cache` |
| Naplók | `*.log` |
| Adatbázis | `*.sqlite`, `*.sqlite3`, `*.db` |

### 2. `.gitignore` támogatás

A projekt gyökerében lévő `.gitignore` fájl mintáit teljes mértékben figyelembe veszi, beleértve:
- Glob minták (`*.generated.go`, `build/`)
- Negáció (`!important.log`)
- Könyvtár-specifikus minták (`docs/internal/`)

### 3. `.claudeignore` támogatás

A `.gitignore`-hoz hasonló szintaxissal működő `.claudeignore` fájl a projekt gyökerében. Ez lehetővé teszi, hogy a kód indexelésből kizárj olyan fájlokat, amelyeket a git-ből nem akarsz kizárni, de a Claude számára nem relevánsak.

Példa `.claudeignore`:
```
# Generált fájlok - Claude-nak nem kell látnia
*.generated.go
*.pb.go

# Teszt fixture-ök (túl nagy fájlok)
testdata/large/

# Régi migráciök
migrations/archive/
```

### 4. CLI `--exclude` minták

Futásidejű kizárás az `--exclude` flag-gel:

```bash
./codeindex-mcp.exe --exclude "*.generated.go" --exclude "vendor/"
```

### 5. Bináris fájl detektálás

A fájl első 512 byte-ját vizsgálja null byte-okra. Ha talál, a fájlt binárisnak tekinti és kihagyja az indexelésből. Ez a `.gitignore`-tól független védelem.

### 6. Fájlméret korlát

Az `--max-file-size` paraméterrel állítható (alapértelmezés: 1 MB). Az ennél nagyobb fájlok kimaradnak.

### Prioritás

A szűrők sorrendje:
1. Beépített minták (legmagasabb prioritás, mindig érvényes)
2. `.gitignore` szabályok
3. `.claudeignore` szabályok
4. CLI `--exclude` minták
5. Bináris detektálás
6. Fájlméret korlát

Ha bármelyik szűrő igaz, a fájl kimarad.

## Architektúra

```
Claude Code (stdio) <──> MCP Server <──> Index Engine
                                            │
                                    ┌───────┼────────┐
                                    │       │        │
                                  Bleve   FileMap   Watcher
                               (full-text) (path)  (fsnotify)
```

### Két párhuzamos index

| Index | Technológia | Funkció |
|-------|-------------|---------|
| **Content Index** | Bleve `NewMemOnly()` | Full-text keresés fájltartalomban (inverted index) |
| **File Path Index** | Go `map` + sorted slice | Fájlnév/útvonal keresés glob mintákkal |

### File Watcher

- **fsnotify** könyvtár, Windows-on `ReadDirectoryChangesW` API-t használ
- Rekurzív: induláskor minden alkönyvtárat figyel
- **100ms debounce ablak**: az editorok (VS Code, stb.) mentéskor több event-et generálnak, ezeket összevonja
- Új könyvtár létrehozásakor automatikusan hozzáadja a watcherhez
- `.gitignore` vagy `.claudeignore` változásakor automatikusan újratölti a szűrési szabályokat

### Indulási folyamat

1. CLI flagek feldolgozása
2. Ignore matcher létrehozása (beépített + .gitignore + .claudeignore + CLI minták)
3. Bleve in-memory index és file path index inicializálása
4. Párhuzamos indexelés 8 worker goroutine-nal
5. File watcher indítása
6. MCP szerver indítása stdio transport-on

## Projekt struktúra

```
codeindex-mcp/
├── main.go                  # Belépési pont, CLI flagek, komponensek összekötése
├── go.mod / go.sum          # Go modul definíció
├── CLAUDE.md                # AI-optimalizált kódolási alapelvek
├── .gitignore
├── server/
│   └── server.go            # MCP szerver beállítás, tool regisztráció
├── index/
│   ├── content.go           # Bleve content index (full-text keresés)
│   ├── content_test.go
│   ├── document.go          # IndexedFile struktúra
│   ├── files.go             # File path index (glob keresés)
│   └── files_test.go
├── watcher/
│   ├── watcher.go           # Rekurzív fsnotify wrapper
│   └── debouncer.go         # 100ms event összevonás
├── ignore/
│   ├── ignore.go            # .gitignore + .claudeignore + custom minták
│   ├── ignore_test.go
│   └── defaults.go          # Beépített ignore minták
├── tools/
│   ├── search.go            # codeindex_search tool handler
│   ├── files.go             # codeindex_files tool handler
│   ├── status.go            # codeindex_status tool handler
│   ├── reindex.go           # codeindex_reindex tool handler
│   └── format.go            # Kimenet formázás
└── language/
    ├── detect.go            # Kiterjesztés → nyelv leképezés (70+)
    ├── detect_test.go
    ├── binary.go            # Bináris fájl detektálás
    └── binary_test.go
```

## Függőségek

| Könyvtár | Verzió | Funkció |
|----------|--------|---------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | v1.3.0 | MCP szerver (stdio transport) |
| [blevesearch/bleve/v2](https://github.com/blevesearch/bleve) | v2.5.7 | In-memory full-text search |
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | File system watching |
| [bmatcuk/doublestar/v4](https://github.com/bmatcuk/doublestar) | v4.10.0 | `**` glob support |
| [denormal/go-gitignore](https://github.com/denormal/go-gitignore) | latest | .gitignore / .claudeignore parsing |

## Teljesítmény

| Metrika | ~5k fájl | ~10k fájl |
|---------|----------|-----------|
| Indulási indexelés | ~1-2s | ~2-3s |
| Memóriahasználat | ~75-100 MB | ~180-230 MB |
| Szöveges keresés | <5ms | <10ms |
| Regex keresés | <50ms | <50ms |
| Glob keresés | <2ms | <5ms |
| Inkrementális update | <10ms/fájl | <10ms/fájl |

## Támogatott nyelvek

A nyelv-felismerés 70+ kiterjesztést ismer fel, többek között:

Go, TypeScript, JavaScript, Python, Rust, Java, Kotlin, C, C++, C#, Swift, Dart, Ruby, PHP, Shell, PowerShell, HTML, CSS, SCSS, Sass, Less, JSON, YAML, TOML, XML, SQL, GraphQL, Protobuf, Terraform, Lua, R, Scala, Elixir, Erlang, Haskell, Zig, Vue, Svelte, Markdown, Dockerfile, Makefile, CMake, Batch, és még sok más.

## Licenc

MIT
