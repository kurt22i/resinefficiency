package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var referencesim = "" //link to the gcsim that gives rotation, er reqs and optimization priority
//var chars = make([]Character, 4);
var artifarmtime = 126 //how long it should simulate farming artis, set as number of artifacts farmed. 20 resin ~= 1.07 artifacts.
var artifarmsims = -1  //default: -1, which will be 100000/artifarmtime. set it to something else if desired.
//var domains []string = {"esf"}
var simspertest = 10000 //iterations to run gcsim at when testing dps gain from upgrades.
var godatafile = ""     //filename of the GO data that will be used for weapons, current artifacts, and optimization settings besides ER. When go adds ability to optimize for x*output1 + y*output2, the reference sim will be used to determine optimization target.

func main() {
	/*var d bool
	var force bool
	flag.BoolVar(&d, "d", false, "skip re-download executable?")
	flag.BoolVar(&force, "f", false, "force rerun all")
	flag.Parse()*/

	err := run()

	if err != nil {
		fmt.Printf("Error encountered, ending script: %+v\n", err)
	}

	fmt.Print("\ntesting complete (press enter to exit)")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func run() error {

	if true {
		//download nightly cmd line build
		//https://github.com/genshinsim/gcsim/releases/download/nightly/gcsim.exe
		err := download("./gcsim.exe", "https://github.com/genshinsim/gcsim/releases/latest/download/gcsim.exe")
		if err != nil {
			return errors.Wrap(err, "")
		}
	}

	//get json data from url
	data := readURL(referencesim)

	//get baseline result
	baseline := runTest(test{"baseline", []int{0}}, data.Config)
	printResult(baseline, nil)

	//generate necessary tests
	tests := getTests(data)

	//run and print the tests (simultaneously, so that users can see data gradually coming in)
	for _, t := range tests {
		printResult(runTest(t, data.Config), baseline)
	}

	return nil
}

func download(path string, url string) error {
	//remove if exists
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		fmt.Printf("%v already exists, deleting...\n", path)
		// path/to/whatever exists
		os.RemoveAll(path)
	}

	fmt.Printf("Downloading: %v\n", url)
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return errors.Wrap(err, "")
}

/*type pack struct {
	Author      string `yaml:"author" json:"author"`
	Config      string `yaml:"config" json:"config"`
	Description string `yaml:"description" json:"description"`
	//the following are machine generated fields
	Hash      string  `yaml:"hash" json:"hash"`
	Team      []char  `yaml:"team" json:"team"`
	DPS       float64 `yaml:"dps" json:"dps"`
	Mode      string  `yaml:"mode" json:"mode"`
	Duration  float64 `yaml:"duration" json:"duration"`
	NumTarget int     `yaml:"target_count" json:"target_count"`
	ViewerKey string  `yaml:"viewer_key" json:"viewer_key"`
	//unexported stuff
	gzPath   string
	filepath string
	changed  bool
}*/

/*type char struct {
	Name    string       `yaml:"name" json:"name"`
	Con     int          `yaml:"con" json:"con"`
	Weapon  string       `yaml:"weapon" json:"weapon"`
	Refine  int          `yaml:"refine" json:"refine"`
	ER      float64      `yaml:"er" json:"er"`
	Talents TalentDetail `yaml:"talents" json:"talents"`
}*/

type TalentDetail struct {
	Attack int `json:"attack"`
	Skill  int `json:"skill"`
	Burst  int `json:"burst"`
}

type jsondata struct {
	Config     string `json:"config"`
	Characters []struct {
		Name   string `json:"name"`
		Level  int    `json:"level"`
		MaxLvl int    `json:"max_level"`
		Cons   int    `json:"cons"`
		Weapon struct {
			Name   string `json:"name"`
			Refine int    `json:"refine"`
			Level  int    `json:"level"`
			MaxLvl int    `json:"max_level"`
		} `json:"weapon"`
		Stats   []float64    `json:"stats"`
		Talents TalentDetail `json:"talents"`
	} `json:"char_details"`
	DPS       float64 `json:"dps"`
	NumTarget int     `json:"target_count"`
}

/*type result struct {
	Duration FloatResult `json:"sim_duration"`
	DPS      FloatResult `json:"dps"`
	Targets  []struct {
		Level int `json:"level"`
	} `json:"target_details"`
	Characters []struct {
		Name   string `json:"name"`
		Cons   int    `json:"cons"`
		Weapon struct {
			Name   string `json:"name"`
			Refine int    `json:"refine"`
		} `json:"weapon"`
		Stats   []float64    `json:"stats"`
		Talents TalentDetail `json:"talents"`
	} `json:"char_details"`
}*/

type result struct {
	info  string
	DPS   float64
	resin float64
}

type test struct {
	typ    string
	params []int
}

var reIter = regexp.MustCompile(`iteration=(\d+)`)
var reWorkers = regexp.MustCompile(`workers=(\d+)`)

func readURL(url string) (data2 jsondata) {
	spaceClient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	//req.Header.Set("User-Agent", "spacecount-tutorial") uh is this part important? i hope not

	res, getErr := spaceClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	data := jsondata{}
	err2 := json.Unmarshal(body, &data)
	if err2 != nil {
		fmt.Println(err2)
		return
	}

	//fix the iterations
	data.Config = reIter.ReplaceAllString(data.Config, "iteration=10000") //should be using the simspertest variable for this
	data.Config = reWorkers.ReplaceAllString(data.Config, "workers=30")

	return data
}

/*func loadData(dir string) ([]pack, error) {
	var data []pack

	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "")
		}
		//do nothing if is directory
		if info.IsDir() {
			return nil
		}
		fmt.Printf("\tReading file: %v at %v\n", info.Name(), path)
		file, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "")
		}
		var d pack
		err = yaml.Unmarshal(file, &d)
		if err != nil {
			return errors.Wrap(err, "")
		}
		d.filepath = path

		data = append(data, d)

		return nil
	})

	return data, err
}*/

func runTest(t test, config string) (res result) {
	var simdata jsondata

	switch t.typ {
	case "baseline":
		simdata = runSim(config)
	case "level":
		simdata = runSim(runLevelTest(t, config))
	case "talent":
		simdata = runSim(runTalentTest(t, config))
	case "weapon":
		simdata = runSim(runWeaponTest(t, config))
	//case "artifact" in future... hopefully...
	default:
		fmt.Printf("invalid test type %v??", t.typ)
	}

	return generateResult(t, simdata)
}

func generateResult(t test, sd jsondata) (res2 result) {
	return result{desc(t, sd),sd.DPS,resin(t)}
}

func desc(t test, sd jsondata) (dsc string) {
	switch t.typ {
	case "baseline":
		return "current"
	case "level":
		return sd.Characters[t.params[0]].Name + " to " + strconv.Itoa(t.params[1]) + "/" + strconv.Itoa(t.params[2])
	case "talent":
		talent := "aa"
		if(t.params[1]==1) {
			talent = "e"
		} else if(t.params[1]==2) {
			talent = "q"
		}
		ascension := ""
		if(t.params[3]==1) {
			ascension = " (requires ascension)"
		}
		return sd.Characters[t.params[0]].Name + " " + talent + " to " + strconv.Itoa(t.params[2]) + ascension
	case "weapon":
		return sd.Characters[t.params[0]].Name + "'s " + sd.Characters[t.params[0]].Weapon.Name + " to " + strconv.Itoa(t.params[2]) + "/" + strconv.Itoa(t.params[3])
	//case "artifact" in future... hopefully...
	default:
		fmt.Printf("invalid test type %v??", t.typ)
	}
	return ""
}

type materials struct {
	info  string
	DPS   float64
	resin float64
}

resin(t test) {

}

//these 3 test functions below should probably go in a diff file
func runLevelTest(t test, config string) (c string) { //params for level test: 0: charid 1: new level 2: new max level
	lines := strings.Split(config, "\n")
	count := 0
	curline := -1
	for count <= t.params[0] {
		curline++
		if strings.Contains(lines[curline], "char lvl") {
			count++
		}
	}

	newline := lines[curline][0 : strings.Index(lines[curline], "lvl")+4]
	newline += strconv.Itoa(t.params[1]) + "/" + strconv.Itoa(t.params[2])
	newline += lines[curline][strings.Index(lines[curline], " cons"):]
	lines[curline] = newline

	return strings.Join(lines, "\n")
}

func runTalentTest(t test, config string) (c string) { //params for talent test: 0: charid 1: talent id (aa/e/q) 2: new level 3: requires ascension (0 or 1) 4: new level (if needed) 5: new max level (if needed)
	cfg := config
	if t.params[3] == 1 { //increase the level if upgrading the talent requires ascension
		cfg = runLevelTest(test{"thisshouldntmatter", []int{t.params[0], t.params[4], t.params[5]}}, config)
	}

	lines := strings.Split(cfg, "\n")
	count := 0
	curline := -1
	for count <= t.params[0] {
		curline++
		if strings.Contains(lines[curline], "char lvl") {
			count++
		}
	}

	//10 makes this really annoying because it's two chars instead of one
	start := strings.Index(lines[curline], "talent=") + 6
	end := strings.Index(lines[curline], ",")

	if t.params[1] >= 1 { // upgrading e talent
		start = end
		end = start + strings.Index(lines[curline][start+1:], ",") + 1
	}

	if t.params[1] == 2 { //upgrading q talent
		start = end
		end = start + strings.Index(lines[curline][start+1:], ",") + 1
	}

	newline := lines[curline][:start]
	newline += strconv.Itoa(t.params[2])
	newline += lines[curline][end:]
	lines[curline] = newline

	return strings.Join(lines, "\n")
}

func runWeaponTest(t test, config string) (c string) { //params for weapon test: 0: charid 1: weapon *s 2: new level 3: new max level
	lines := strings.Split(config, "\n")
	count := 0
	curline := -1
	for count <= t.params[0] {
		curline++
		if strings.Contains(lines[curline], "refine=") {
			count++
		}
	}

	newline := lines[curline][0 : strings.Index(lines[curline], "lvl")+4]
	newline += strconv.Itoa(t.params[2]) + "/" + strconv.Itoa(t.params[3]) + ";"
	lines[curline] = newline

	return strings.Join(lines, "\n")
}

/*func process(data []pack, latest string, force bool) error {
	//make a tmp folder if it doesn't exist
	if _, err := os.Stat("./tmp"); !os.IsNotExist(err) {
		fmt.Println("tmp folder already exists, deleting...")
		// path/to/whatever exists
		os.RemoveAll("./tmp/")
	}
	os.Mkdir("./tmp", 0755)

	fmt.Println("Rerunning configs...")

	for i := range data {
		//compare hash vs current hash; if not the same rerun
		if !force && data[i].Hash == latest {
			fmt.Printf("\tSkipping %v\n", data[i].filepath)
			continue
		}
		data[i].changed = true
		//re run sim
		fmt.Printf("\tRerunning %v\n", data[i].filepath)
		outPath := fmt.Sprintf("./tmp/%v", time.Now().Nanosecond())
		err := runSim(data[i].Config, outPath)
		if err != nil {
			return errors.Wrap(err, "")
		}
		//read the json and populate
		data[i].Hash = latest
		jsonData, err := os.ReadFile(outPath + ".json")
		if err != nil {
			return errors.Wrap(err, "")
		}
		readResultJSON(jsonData, &data[i])

		//find the mode
		match := reMode.FindStringSubmatch(data[i].Config)
		if match != nil {
			data[i].Mode = match[1]
		}

		//overwrite yaml
		out, err := yaml.Marshal(data[i])
		if err != nil {
			return errors.Wrap(err, "")
		}
		os.Remove(data[i].filepath)
		err = os.WriteFile(data[i].filepath, out, 0755)
		if err != nil {
			return errors.Wrap(err, "")
		}

		//write gz
		writeJSONtoGZ(jsonData, outPath)

		data[i].gzPath = outPath + ".gz"
	}

	return nil
}*/

/*func readResultJSON(jsonData []byte, p *pack) error {

	var r result
	err := json.Unmarshal(jsonData, &r)
	if err != nil {
		return errors.Wrap(err, "")
	}

	p.DPS = r.DPS.Mean
	p.Duration = r.Duration.Mean
	p.NumTarget = len(r.Targets)

	p.Team = make([]char, 0, len(r.Characters))

	//team info
	for _, v := range r.Characters {
		var c char
		c.Name = v.Name
		c.Con = v.Cons
		c.Weapon = v.Weapon.Name
		c.Refine = v.Weapon.Refine
		c.Talents = v.Talents

		//grab er stats
		c.ER = v.Stats[ERIndex]

		p.Team = append(p.Team, c)
	}
	return nil
}*/

func writeJSONtoGZ(jsonData []byte, fpath string) error {
	f, err := os.OpenFile(fpath+".gz", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	zw.Write(jsonData)
	err = zw.Close()
	return errors.Wrap(err, "")
}

func getVersion() (string, error) {
	fmt.Println("Getting last hash...")
	out, err := exec.Command("./gcsim", "-version").Output()
	hash := strings.Trim(string(out), "\n")
	fmt.Printf("Latest hash: %v\n", hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func runSim(cfg string) (data2 jsondata) {
	path := fmt.Sprintf("./tmp/%v", time.Now().Nanosecond())

	//write config to file
	err := os.WriteFile(path+".txt", []byte(cfg), 0755)
	if err != nil {
		fmt.Printf("error saving config file: %v\n", err)
	}
	out, err := exec.Command("./gcsim", "-c", path+".txt", "-out", path+".json").Output()

	if err != nil {
		fmt.Printf("%v\n", string(out))
	}

	jsn, err := os.ReadFile(path + ".json")

	data := jsondata{}
	err2 := json.Unmarshal(jsn, &data)
	if err2 != nil {
		fmt.Println(err2)
		return
	}

	return data
}
