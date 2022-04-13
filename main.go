package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
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

const PurpleBookXP = 20000

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
	return result{desc(t, sd), sd.DPS, resin(t)}
}

func desc(t test, sd jsondata) (dsc string) {
	switch t.typ {
	case "baseline":
		return "current"
	case "level":
		return sd.Characters[t.params[0]].Name + " to " + strconv.Itoa(t.params[1]) + "/" + strconv.Itoa(t.params[2])
	case "talent":
		talent := "aa"
		if t.params[1] == 1 {
			talent = "e"
		} else if t.params[1] == 2 {
			talent = "q"
		}
		ascension := ""
		if t.params[3] == 1 {
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

var talentmora = []int{125, 175, 250, 300, 375, 1200, 2600, 4500, 7000}
var talentbks = []int{3, 6, 12, 18, 27, 36, 54, 108, 144}

type materials struct {
	mora        int
	books       float64 //measured in purple books
	bossmats    int     //for example, hoarfrost core. Currently we assume gemstones aren't important/worth counting resin for because of azoth dust, but in the future we should have options instead of assumptions.
	talentbooks int     //measured in teachings
	weaponmats  int     //measured in the lowest level
	artifacts   int     //not used yet
}

var wpmats = [][]int{{}, {}, {5, 5 * 3, 9 * 3, 5 * 9, 9 * 9, 6 * 27}}
var wpmora = [][]int{{}, {}, {10, 20, 30, 45, 55, 65}}

func resin(t test, sd jsondata) (rsn float64) {
	mats := materials{0, 0.0, 0, 0, 0, 0}

	switch t.typ {
	case "baseline":
		return -1
	case "level":
		mats.books += xptolvl(sd.Characters[t.params[0]].Level, t.params[1]) / PurpleBookXP
		mats.mora = int(math.Floor(mats.books / 5.0))
		if t.params[2] != sd.Characters[t.params[0]].MaxLvl { //if we ascended
			mats.mora += 20000 * (t.params[2] - 30) / 10
			mats.bossmats += int(math.Floor((float64(t.params[2])-30.0)/10.0*(float64(t.params[2])-30.0)/10.0/2.0)) + int(math.Max(0, float64(t.params[2])-80.0)/5.0)
		}
	case "talent":
		mats.mora += talentmora[t.params[2]-2] * 100
		mats.talentbooks += talentbks[t.params[2]-2]
		if t.params[3] == 1 {
			mats.books += xptolvl(sd.Characters[t.params[0]].Level, t.params[1]) / PurpleBookXP
			mats.mora += int(math.Floor(mats.books / 5.0))
			mats.mora += 20000 * (t.params[2] - 30) / 10
			mats.bossmats += int(math.Floor((float64(t.params[2])-30.0)/10.0*(float64(t.params[2])-30.0)/10.0/2.0)) + int(math.Max(0, float64(t.params[2])-80.0)/5.0)
		}
	case "weapon":
		mats.mora += (wpexp[t.params[1]-3][t.params[2]-1] - wpexp[t.params[1]-3][sd.Characters[t.params[0]].Weapon.Level-1]) / 10
		if t.params[3] != sd.Characters[t.params[0]].Weapon.MaxLvl { //if we ascended
			mats.weaponmats += wpmats[t.params[1]-3][(t.params[3]-30)/10-1]
			mats.mora += wpmora[t.params[1]-3][(t.params[3]-30)/10-1]
		}
	//case "artifact" in future... hopefully...
	default:
		fmt.Printf("invalid test type %v??", t.typ)
	}

	return resinformats(mats)
}

func xptolvl(l1 int, l2 int) (xp float64) {
	return float64(crexp[l2] - crexp[l1])
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

var crexp = []int{
	0,
	1000,
	2325,
	4025,
	6175,
	8800,
	11950,
	15675,
	20025,
	25025,
	30725,
	37175,
	44400,
	52450,
	61375,
	71200,
	81950,
	93675,
	106400,
	120175,
	135050,
	151850,
	169850,
	189100,
	209650,
	231525,
	254775,
	279425,
	305525,
	333100,
	362200,
	392850,
	425100,
	458975,
	494525,
	531775,
	570750,
	611500,
	654075,
	698500,
	744800,
	795425,
	848125,
	902900,
	959800,
	1018875,
	1080150,
	1143675,
	1209475,
	1277600,
	1348075,
	1424575,
	1503625,
	1585275,
	1669550,
	1756500,
	1846150,
	1938550,
	2033725,
	2131725,
	2232600,
	2341550,
	2453600,
	2568775,
	2687100,
	2808625,
	2933400,
	3061475,
	3192875,
	3327650,
	3465825,
	3614525,
	3766900,
	3922975,
	4082800,
	4246400,
	4413825,
	4585125,
	4760350,
	4939525,
	5122700,
	5338925,
	5581950,
	5855050,
	6161850,
	6506450,
	6893400,
	7327825,
	7815450,
	8362650,
}

var wpexp = [][]int{
	{
		0,
		275,
		700,
		1300,
		2100,
		3125,
		4400,
		5950,
		7800,
		9975,
		12475,
		15350,
		18600,
		22250,
		26300,
		30800,
		35750,
		41150,
		47050,
		53475,
		60400,
		68250,
		76675,
		85725,
		95400,
		105725,
		116700,
		128350,
		140700,
		153750,
		167550,
		182075,
		197375,
		213475,
		230375,
		248075,
		266625,
		286025,
		306300,
		327475,
		349525,
		373675,
		398800,
		424925,
		452075,
		480275,
		509525,
		539850,
		571275,
		603825,
		637475,
		674025,
		711800,
		750800,
		791075,
		832625,
		875475,
		919625,
		965125,
		1011975,
		1060200,
		1112275,
		1165825,
		1220875,
		1277425,
		1335525,
		1395175,
		1456400,
		1519200,
		1583600,
		1649625,
		1720700,
		1793525,
		1868100,
		1944450,
		2022600,
		2102600,
		2184450,
		2268150,
		2353725,
		2441225,
		2544500,
		2660575,
		2791000,
		2937500,
		3102050,
		3286825,
		3494225,
		3727000,
		3988200,
	},
	{
		0,
		400,
		1025,
		1925,
		3125,
		4675,
		6625,
		8975,
		11775,
		15075,
		18875,
		23225,
		28150,
		33675,
		39825,
		46625,
		54125,
		62325,
		71275,
		81000,
		91500,
		103400,
		116175,
		129875,
		144525,
		160150,
		176775,
		194425,
		213125,
		232900,
		253800,
		275825,
		299025,
		323400,
		349000,
		375825,
		403925,
		433325,
		464050,
		496125,
		529550,
		566125,
		604200,
		643800,
		684950,
		727675,
		772000,
		817950,
		865550,
		914850,
		965850,
		1021225,
		1078450,
		1137550,
		1198575,
		1261525,
		1326450,
		1393350,
		1462275,
		1533250,
		1606300,
		1685200,
		1766325,
		1849725,
		1935425,
		2023450,
		2113825,
		2206575,
		2301725,
		2399300,
		2499350,
		2607025,
		2717350,
		2830350,
		2946050,
		3064475,
		3185675,
		3309675,
		3436500,
		3566175,
		3698750,
		3855225,
		4031100,
		4228700,
		4450675,
		4699975,
		4979925,
		5294175,
		5646875,
		6042650,
	},
	{
		0,
		600,
		1550,
		2900,
		4700,
		7025,
		9950,
		13475,
		17675,
		22625,
		28325,
		34850,
		42250,
		50550,
		59775,
		69975,
		81225,
		93525,
		106950,
		121550,
		137300,
		155150,
		174325,
		194875,
		216850,
		240300,
		265250,
		291725,
		319775,
		349450,
		380800,
		413850,
		448650,
		485225,
		523625,
		563875,
		606025,
		650125,
		696225,
		744350,
		794500,
		849375,
		906500,
		965900,
		1027625,
		1091725,
		1158225,
		1227150,
		1298550,
		1372500,
		1449000,
		1532075,
		1617925,
		1706575,
		1798125,
		1892550,
		1989950,
		2090300,
		2193700,
		2300175,
		2409750,
		2528100,
		2649800,
		2774900,
		2903450,
		3035500,
		3171075,
		3310200,
		3452925,
		3599300,
		3749375,
		3910900,
		4076400,
		4245900,
		4419450,
		4597100,
		4778900,
		4964900,
		5155150,
		5349675,
		5548550,
		5783275,
		6047100,
		6343500,
		6676475,
		7050425,
		7470350,
		7941725,
		8470775,
		9064450,
	},
}
