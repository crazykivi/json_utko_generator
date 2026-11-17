package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Route struct {
	PartNumber    int     `json:"PartNumber"`
	ObjectId      string  `json:"ObjectId"`
	ObjectName    string  `json:"ObjectName"`
	ObjectType    string  `json:"ObjectType"`
	Power         N       `json:"Power"`
	Volume        N       `json:"Volume"`
	Latitude      N       `json:"Latitude"`
	Longitude     N       `json:"Longitude"`
	Polyline      *string `json:"Polyline"`
	Distance      N       `json:"Distance"`
	Tkm           N       `json:"Tkm"`
	TravelTime    N       `json:"TravelTime"`
	Speed         N       `json:"Speed"`
	Availability  int     `json:"Availability"`
	TransportType int     `json:"TransportType"`
	Route         *Route  `json:"Route,omitempty"`
}

type Population struct {
	PopulationId   string `json:"PopulationId"`
	PopulationName string `json:"PopulationName"`
	CountWaste     N      `json:"CountWaste"`
	Latitude       N      `json:"Latitude"`
	Longitude      N      `json:"Longitude"`
	Route          *Route `json:"Route"`
}

type Result struct {
	Routes []Population `json:"Routes"`
}

type N struct {
	f float64
	j string
}

func (n N) MarshalJSON() ([]byte, error) {
	if n.j == "" {
		return []byte("0"), nil
	}
	return []byte(n.j), nil
}

func pyFloat(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	sci := strconv.FormatFloat(v, 'e', -1, 64)
	exp, _ := strconv.Atoi(sci[strings.IndexByte(sci, 'e')+1:])
	if exp < -4 || exp >= 16 {
		return sci
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func roundPy(v float64, digits int) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', digits, 64), 64)
	if err != nil {
		return 0
	}
	return r
}

func pyID(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '0' && len(s) > 1 && s[1] != '.' {
		return s
	}
	if strings.IndexFunc(s, func(r rune) bool { return !strings.ContainsRune("0123456789.eE+-", r) }) >= 0 {
		return s
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return s
	}
	return pyFloat(v)
}

var objectTypes = map[string]string{
	"размещение":     "DisposalObject",
	"обработка":      "AssortmentObject",
	"обезвреживание": "IncinerationObject",
	"утилизация":     "CompostingObject",
	"хранение":       "StorageObject",
	"перегрузка":     "TransshipmentObject",
}

const (
	skipRows = 2
	legWidth = 9
	leg1Base = 6
	leg2Base = 15
	leg3Base = 24
)

var stdin = bufio.NewReader(os.Stdin)

func cell(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

func objectType(s string) string {
	if t, ok := objectTypes[s]; ok {
		return t
	}
	return "UnknownObject"
}

type parser struct{ err error }

func (p *parser) num(s string) N {
	if s == "" || p.err != nil {
		return N{0, "0"}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		p.err = fmt.Errorf("could not convert string to float: '%s'", s)
		return N{0, "0"}
	}
	return N{v, pyFloat(v)}
}

func (p *parser) div(a, b N) N {
	if b.f == 0 || math.IsNaN(a.f) || math.IsInf(a.f, 0) || math.IsInf(b.f, 0) {
		return N{0, "0"}
	}
	v := roundPy(a.f/b.f, 2)
	return N{v, pyFloat(v)}
}

func (p *parser) leg(row []string, base, part int, strict bool) *Route {
	var v [legWidth]string
	for i := range v {
		v[i] = cell(row, base+i)
	}
	if strict {
		for _, s := range v {
			if s == "" {
				return nil
			}
		}
	}
	dist, speed := p.num(v[6]), p.num(v[8])
	tt := p.div(dist, speed)
	return &Route{
		PartNumber:    part,
		ObjectId:      pyID(v[0]),
		ObjectName:    fmt.Sprintf("%d Плечо", part),
		ObjectType:    objectType(v[1]),
		Power:         p.num(v[2]),
		Volume:        p.num(v[3]),
		Latitude:      p.num(v[4]),
		Longitude:     p.num(v[5]),
		Distance:      dist,
		Tkm:           p.num(v[7]),
		TravelTime:    tt,
		Speed:         speed,
		Availability:  1,
		TransportType: 1,
	}
}

func emptyRow(row []string) bool {
	for _, s := range row {
		if s != "" {
			return false
		}
	}
	return true
}

func convert(path, sheet string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть '%s': %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if sheet == "" {
		sheet = f.GetSheetName(0)
	}
	if sheet == "" {
		return nil, fmt.Errorf("в книге нет листов")
	}

	rows, err := f.Rows(sheet)
	if err != nil {
		return nil, fmt.Errorf("лист '%s': %w", sheet, err)
	}
	defer func() { _ = rows.Close() }()

	raw := excelize.Options{RawCellValue: true}

	res := &Result{Routes: make([]Population, 0, 1024)}
	rowNum, lastData := 0, 0

	for rows.Next() {
		rowNum++
		if rowNum <= skipRows {
			continue
		}
		row, err := rows.Columns(raw)
		if err != nil {
			fmt.Printf("Ошибка в строке %d: %v\n", rowNum, err)
			continue
		}

		p := &parser{}
		r1 := p.leg(row, leg1Base, 1, false)
		r2 := p.leg(row, leg2Base, 2, true)
		r3 := p.leg(row, leg3Base, 3, true)
		if r2 != nil {
			r2.Route = r3
		}
		r1.Route = r2

		name := ""
		if cell(row, 1) != "" {
			name = cell(row, 2)
		}

		pop := Population{
			PopulationId:   pyID(cell(row, 0)),
			PopulationName: name,
			CountWaste:     p.num(cell(row, 5)),
			Latitude:       p.num(cell(row, 3)),
			Longitude:      p.num(cell(row, 4)),
			Route:          r1,
		}
		if p.err != nil {
			fmt.Printf("Ошибка в строке %d: %v\n", rowNum, p.err)
			continue
		}

		res.Routes = append(res.Routes, pop)
		if !emptyRow(row) {
			lastData = len(res.Routes)
		}
	}
	if err := rows.Error(); err != nil {
		return nil, fmt.Errorf("чтение листа '%s': %w", sheet, err)
	}
	res.Routes = res.Routes[:lastData]
	return res, nil
}

type noTrailingNL struct{ w io.Writer }

func (t noTrailingNL) Write(p []byte) (int, error) {
	if n := len(p); n > 0 && p[n-1] == '\n' {
		if _, err := t.w.Write(p[:n-1]); err != nil {
			return 0, err
		}
		return n, nil
	}
	return t.w.Write(p)
}

func writeJSON(path string, res *Result) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(noTrailingNL{w})
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err = enc.Encode(res); err != nil {
		return err
	}
	return w.Flush()
}

func main() {
	setConsoleUTF8()

	out := flag.String("o", "", "выходной JSON-файл (по умолчанию <имя>.json)")
	sheet := flag.String("sheet", "", "имя листа (по умолчанию первый)")
	all := flag.Bool("all", false, "обработать все .xlsx в папке программы")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Конвертер Excel → JSON\n\n"+
			"  gisutko.exe\n"+
			"  gisutko.exe data.xlsx -o result.json\n"+
			"  gisutko.exe -all\n\nФлаги:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	interactive := flag.NArg() == 0 && !*all
	if interactive {
		defer pause()
	}

	var err error
	switch {
	case *all:
		err = processAll(*sheet)
	case flag.NArg() > 0:
		err = process(flag.Arg(0), *out, *sheet)
	default:
		err = runInteractive(*out, *sheet)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if !interactive {
			os.Exit(1)
		}
	}
}

func process(input, out, sheet string) error {
	if out == "" {
		out = defaultOut(input)
	}
	if !strings.EqualFold(filepath.Ext(out), ".json") {
		out += ".json"
	}
	res, err := convert(input, sheet)
	if err != nil {
		return err
	}
	if err := writeJSON(out, res); err != nil {
		return err
	}
	fmt.Printf("JSON успешно сохранён в '%s'\n", out)
	return nil
}

func processAll(sheet string) error {
	dir := baseDir()
	files, err := xlsxFiles(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("В текущей папке нет Excel-файлов (.xlsx)")
	}
	failed := 0
	for _, name := range files {
		if err := process(filepath.Join(dir, name), "", sheet); err != nil {
			failed++
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("файлов с ошибками: %d из %d", failed, len(files))
	}
	return nil
}

func runInteractive(out, sheet string) error {
	dir := baseDir()
	files, err := xlsxFiles(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("В текущей папке нет Excel-файлов (.xlsx)")
	}

	fmt.Println("Найденные Excel-файлы:")
	for i, name := range files {
		fmt.Printf("%d. %s\n", i+1, name)
	}
	fmt.Print("Введите номер файла для обработки: ")
	line, _ := stdin.ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(files) {
		return fmt.Errorf("Неверный выбор.")
	}
	input := filepath.Join(dir, files[n-1])

	if out == "" {
		def := filepath.Base(defaultOut(input))
		fmt.Printf("Введите имя для JSON-файла (Enter — по умолчанию '%s'): ", def)
		name, _ := stdin.ReadString('\n')
		out = strings.TrimSpace(name)
		if out == "" {
			out = def
		}
		if !filepath.IsAbs(out) {
			out = filepath.Join(dir, out)
		}
	}

	fmt.Printf("\nНачинаю обработку файла '%s'...\n", filepath.Base(input))
	return process(input, out, sheet)
}

func defaultOut(input string) string {
	return strings.TrimSuffix(input, filepath.Ext(input)) + ".json"
}

func baseDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if !strings.HasPrefix(dir, os.TempDir()) {
			return dir
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func xlsxFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "~$") {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".xlsx") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func pause() {
	fmt.Print("\nНажмите Enter, чтобы закрыть окно...")
	_, _ = stdin.ReadString('\n')
}
