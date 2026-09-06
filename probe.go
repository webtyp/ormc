package ormc

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"webtyp.com/fmt"
	"webtyp.com/model"
)

type ProbeRunner func(mainContent string, workDir string) (string, error)

func (g *Generator) runProbe(probes []fieldProbe, infos []StructInfo) ([]probeResult, error) {
	// Sort probes by expression to ensure deterministic cache key
	sort.Slice(probes, func(i, j int) bool {
		return probes[i].expr < probes[j].expr
	})

	// Check cache
	cacheKey := g.getCacheKey(probes)
	if results, ok := probeCache[cacheKey]; ok {
		return results, nil
	}

	// Generate probe source
	pkgs := make(map[string]string) // path -> alias
	var pkgPaths []string
	for _, p := range probes {
		if _, ok := pkgs[p.pkgPath]; !ok {
			alias := fmt.Sprintf("k%d", len(pkgPaths))
			pkgs[p.pkgPath] = alias
			pkgPaths = append(pkgPaths, p.pkgPath)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("package main\n\nimport (\n\t\"fmt\"\n")
	for _, path := range pkgPaths {
		buf.WriteString(fmt.Sprintf("\t%s \"%s\"\n", pkgs[path], path))
	}
	buf.WriteString(")\n\nfunc main() {\n")
	for i, p := range probes {
		// Rewrite expr to use alias
		selector, constructor := parseConstructor(p.expr)
		alias := pkgs[p.pkgPath]
		expr := p.expr
		if selector != "" {
			expr = alias + "." + p.expr[len(selector)+1:]
		} else {
			expr = alias + "." + constructor
		}
		buf.WriteString(fmt.Sprintf("\tfmt.Printf(\"%d=%%d\\n\", int(%s.Storage()))\n", i, expr))
	}
	buf.WriteString("}\n")

	runner := g.probeRunner
	if runner == nil {
		runner = defaultProbeRunner
	}

	output, err := runner(buf.String(), g.rootDir)
	if err != nil {
		return nil, fmt.Err(fmt.Sprintf("probe failure: %v\nOutput:\n%s", err, output))
	}

	// Parse output
	var results []probeResult
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		idx, _ := strconv.Atoi(parts[0])
		val, _ := strconv.Atoi(parts[1])
		if idx < 0 || idx >= len(probes) {
			continue
		}
		p := probes[idx]
		st := model.FieldType(val)

		if st == model.FieldStruct || st == model.FieldStructSlice {
			return nil, fmt.Err(fmt.Sprintf("field %s (struct %s): custom composition kinds are unsupported (kind %s returned storage %v)",
				infos[p.infoIdx].Fields[p.fieldIdx].ColumnName, infos[p.infoIdx].Name, p.expr, st))
		}

		results = append(results, probeResult{
			infoIdx:  p.infoIdx,
			fieldIdx: p.fieldIdx,
			storage:  st,
		})
	}

	if len(results) != len(probes) {
		return nil, fmt.Err(fmt.Sprintf("probe output mismatch: expected %d lines, got %d", len(probes), len(results)))
	}

	probeCache[cacheKey] = results
	return results, nil
}

// probeCacheKey identifies a probe run by (module root, go.mod content hash,
// sorted constructor-expression set) — a comparable struct, not a
// concatenated/hashed string blob, so each component stays inspectable.
type probeCacheKey struct {
	rootDir   string
	goModHash [sha256.Size]byte
	exprSet   string // probes, already sorted by expr, joined as "expr=pkgPath;..."
}

func (g *Generator) getCacheKey(probes []fieldProbe) probeCacheKey {
	key := probeCacheKey{rootDir: g.rootDir}

	modPath := filepath.Join(g.rootDir, "go.mod")
	if b, err := os.ReadFile(modPath); err == nil {
		key.goModHash = sha256.Sum256(b)
	}

	parts := make([]string, len(probes))
	for i, p := range probes {
		parts[i] = p.expr + "=" + p.pkgPath
	}
	key.exprSet = strings.Join(parts, ";")
	return key
}

var probeCache = make(map[probeCacheKey][]probeResult)

func defaultProbeRunner(mainContent string, workDir string) (string, error) {
	tempDir := filepath.Join(workDir, ".ormcprobe")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	mainPath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = tempDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}
