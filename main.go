package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// var (
// 	verbose = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
// 	name    = kingpin.Arg("name", "Name of user.").Required().String()
// )
//var referencesim = *flag.String("url", "", "your simulation")

var referencesim = "https://gcsim.app/viewer/share/BGznqjs62S9w8qxpxPu7w" //link to the gcsim that gives rotation, er reqs and optimization priority
//var chars = make([]Character, 4);
var artifarmtime = 126 //how long it should simulate farming artis, set as number of artifacts farmed. 20 resin ~= 1.07 artifacts.
var artifarmsims = -1  //default: -1, which will be 100000/artifarmtime. set it to something else if desired.
//var domains []string = {"esf"}
var simspertest = 100000 //iterations to run gcsim at when testing dps gain from upgrades.
var godatafile = ""      //filename of the GO data that will be used for weapons, current artifacts, and optimization settings besides ER. When go adds ability to optimize for x*output1 + y*output2, the reference sim will be used to determine optimization target.
var halp = false

func main() {
	flag.IntVar(&simspertest, "i", 10000, "sim iterations per test")
	flag.BoolVar(&halp, "halp", false, "use gzip instead of zlib")
	flag.StringVar(&referencesim, "url", "", "your simulation")
	flag.Parse()

	time.Now().UnixNano()
	rand.Seed(42)

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
		//err := download("./gcsim.exe", "https://github.com/genshinsim/gcsim/releases/download/nightly/gcsim.exe")
		err := download("./gcsim.exe", "https://github.com/genshinsim/gcsim/releases/latest/download/gcsim.exe")
		if err != nil {
			return errors.Wrap(err, "")
		}
	}

	if referencesim == "" {
		return errors.New("please input your simulation by using url=\"linkhere\"!")
	}

	//make a tmp folder if it doesn't exist
	if _, err := os.Stat("./tmp"); !os.IsNotExist(err) {
		fmt.Println("tmp folder already exists, deleting...")
		// path/to/whatever exists
		os.RemoveAll("./tmp/")
	}
	os.Mkdir("./tmp", 0755)

	//get json data from url
	data := readURL(referencesim)
	fmt.Println("running tests...")

	//get baseline result
	baseline := runTest(test{"baseline", []int{0}}, data.Config, data)
	fmt.Print("\n")
	printResult(baseline, baseline)

	//generate necessary tests
	tests := getTests(data)

	//run and print the tests (simultaneously, so that users can see data gradually coming in)
	for _, t := range tests {
		printResult(runTest(t, data.Config, data), baseline)
	}

	return nil
}

func getTests(data jsondata) (tt []test) { //in future, auto skip tests of talents that are not used in the config
	tests := make([]test, 0)
	for i, c := range data.Characters { //should split this into functions

		//add level tests
		newlevel := (c.Level/10 + 1) * 10
		if newlevel < 40 && newlevel != 20 {
			newlevel += 10
		}
		newmax := newlevel + 10
		if newmax < 40 {
			newmax = 40 //could just do this with math.max, but it require floats for some reason
		}
		if newlevel > 90 {
			newlevel = 90
		}
		//fmt.Printf("%v", c)
		if c.Level < 90 && c.Level != c.MaxLvl { //levelup test
			tests = append(tests, test{"level", []int{i, newlevel, c.MaxLvl}})
		} else if c.Level < 90 { //levelup and ascension test
			newmax -= 10
			tests = append(tests, test{"level", []int{i, newlevel, newmax}})
			newlevel -= 10
		}
		if c.MaxLvl < 90 { //ascension test
			tests = append(tests, test{"level", []int{i, newlevel, newmax}})
		}

		//add talent tests
		talents := []int{c.Talents.Attack, c.Talents.Skill, c.Talents.Burst}
		for j, t := range talents {
			if t == 10 {
				continue
			}
			//talent maxlevel reqs: 2 50, 3/4 60, 5/6 70, 7/8 80, 9/10 90
			lvlneed := 10*(t/2) + 50
			if c.MaxLvl < lvlneed { //talent test that requires ascension
				tests = append(tests, test{"talent", []int{i, j, t + 1, 1, newmax - 10, newmax}})
			} else if c.MaxLvl >= 70 && t < 6 { //new: upgrade talents below 6 to 6 if possible, because 1->2 is too small to accurately say anything
				tests = append(tests, test{"talent", []int{i, j, 6, 0, -2, -2}})
			} else { //talent test without ascension
				tests = append(tests, test{"talent", []int{i, j, t + 1, 0, -2, -2}})
			}
		}

		//add weapon tests
		newlevel = (c.Weapon.Level/10 + 1) * 10
		if newlevel < 40 && newlevel != 20 {
			newlevel += 10
		}
		newmax = newlevel + 10
		if newmax < 40 {
			newmax = 40 //could just do this with math.max, but it require floats for some reason
		}
		if newmax > 90 {
			newmax = 90
		}
		if newlevel > 90 {
			newlevel = 90
		}
		if c.Weapon.Level < 90 && c.Weapon.Level != c.Weapon.MaxLvl { //levelup test
			tests = append(tests, test{"weapon", []int{i, rarity(c.Weapon.Name), newlevel, c.Weapon.MaxLvl}})
		} else if c.Weapon.Level < 90 { //levelup and ascension test
			newmax -= 10
			tests = append(tests, test{"weapon", []int{i, rarity(c.Weapon.Name), newlevel, newmax}})
		}
		if c.Weapon.MaxLvl < 90 { //ascension test
			tests = append(tests, test{"weapon", []int{i, rarity(c.Weapon.Name), newlevel, newmax}})
		}

		//add artifact test
		tests = append(tests, test{"artifact", []int{i}})
	}
	return tests
}

func rarity(wep string) int {
	// for _, w := range weps {
	// 	if w.Name == wep {
	// 		return w.Rarity
	// 	}
	// }
	//fmt.Printf("weapon not found: %v", wep)

	jsn, err := os.ReadFile("./wep/" + wep + ".json")
	if err != nil {
		fmt.Println(err)
	}
	data := wepjson{}
	err = json.Unmarshal(jsn, &data)
	if err != nil {
		fmt.Println(err)
	}
	data.Rarity, err = strconv.Atoi(data.Raritys)
	return data.Rarity
}

func printResult(res, base result) {
	info := res.info + ":"
	dps := "DPS: " + fmt.Sprintf("%.0f", res.DPS)
	if res.resin == -1 {
		fmt.Printf("%-40v%-30v\n", info, dps)
		return
	}
	dps += " (+" + fmt.Sprintf("%.0f", res.DPS-base.DPS) + ")"
	resin := "Resin: " + fmt.Sprintf("%.0f", res.resin)
	dpsresin := "DPS/Resin: " + fmt.Sprintf("%.2f", math.Max((res.DPS-base.DPS)/res.resin, 0.0))
	fmt.Printf("%-40v%-30v%-30v%-24v\n", info, dps, resin, dpsresin)
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

type TalentDetail struct {
	Attack int `json:"attack"`
	Skill  int `json:"skill"`
	Burst  int `json:"burst"`
}

type jsondata struct {
	Config     string `json:"config_file"`
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
	DPSraw    FloatResult `json:"dps"`
	NumTarget int         `json:"target_count"`
	DPS       float64
}

type FloatResult struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
	SD   float64 `json:"sd"`
}

type wepjson struct {
	//Name   string `json:"name"`
	Raritys string `json:"rarity"`
	Rarity  int
}

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

type blah struct {
	Data string `json:"data"`
}

func readURL(url string) (data2 jsondata) {
	spaceClient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	urlreal := "https://viewer.gcsim.workers.dev/" + url[strings.LastIndex(url, "/"):]

	req, err := http.NewRequest(http.MethodGet, urlreal, nil)
	if err != nil {
		log.Fatal(err)
	}

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

	idk := blah{}
	data := jsondata{}
	err = json.Unmarshal(body, &idk)
	b64z := idk.Data
	z, err4 := base64.StdEncoding.DecodeString(b64z)
	if err4 != nil {
		fmt.Println(err4)
		return
	}
	r, err2 := zlib.NewReader(bytes.NewReader(z))
	if err2 != nil {
		r, err2 = gzip.NewReader(bytes.NewReader(z))
		if err != nil {
			fmt.Println(err2)
			return
		}
	}
	resul, err3 := ioutil.ReadAll(r)
	if err3 != nil {
		fmt.Println(err3)
		return
	}
	err = json.Unmarshal(resul, &data)
	data.DPS = data.DPSraw.Mean

	if err != nil {
		fmt.Println(err)
		return
	}

	//fix the iterations
	it := "iteration=" + strconv.Itoa(simspertest)
	data.Config = reIter.ReplaceAllString(data.Config, it)
	data.Config = reWorkers.ReplaceAllString(data.Config, "workers=30")

	return data
}

func runTest(t test, config string, baseline jsondata) (res result) {
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
	case "artifact":
		simdata = runSim(runArtifactTest(t, config))
	default:
		fmt.Printf("invalid test type %v??", t.typ)
	}

	return generateResult(t, simdata, baseline)
}

func generateResult(t test, sd jsondata, base jsondata) (res2 result) {
	return result{desc(t, sd), sd.DPS, resin(t, base)}
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

var wpmats = [][]int{{2, 2 * 3, 4 * 3, 2 * 9, 4 * 9, 3 * 27}, {3, 3 * 3, 6 * 3, 3 * 9, 6 * 9, 4 * 27}, {5, 5 * 3, 9 * 3, 5 * 9, 9 * 9, 6 * 27}}
var wpmora = [][]int{{5, 10, 15, 20, 25, 30}, {5, 15, 20, 30, 35, 45}, {10, 20, 30, 45, 55, 65}}

func resin(t test, sd jsondata) (rsn float64) {
	mats := materials{0, 0.0, 0, 0, 0, 0}

	switch t.typ {
	case "baseline":
		return -1
	case "level":
		mats.books += xptolvl(sd.Characters[t.params[0]].Level-1, t.params[1]-1) / PurpleBookXP
		mats.mora = int(math.Floor(mats.books / 5.0))
		if t.params[2] != sd.Characters[t.params[0]].MaxLvl { //if we ascended
			mats.mora += 20000 * (t.params[2] - 30) / 10
			mats.bossmats += int(math.Floor((float64(t.params[2])-30.0)/10.0*(float64(t.params[2])-30.0)/10.0/2.0)) + int(math.Max(0, float64(t.params[2])-80.0)/5.0)
		}
	case "talent":
		if t.params[2] == 6 { //this whole function is ugly, but this part especially. functionality first tho
			talents := []int{sd.Characters[t.params[0]].Talents.Attack, sd.Characters[t.params[0]].Talents.Skill, sd.Characters[t.params[0]].Talents.Burst}
			for i := talents[t.params[1]] + 1; i <= 6; i++ {
				mats.mora += talentmora[i-2] * 100
				mats.talentbooks += talentbks[i-2]
			}
		} else {
			mats.mora += talentmora[t.params[2]-2] * 100
			mats.talentbooks += talentbks[t.params[2]-2]
			if t.params[3] == 1 {
				mats.books += xptolvl(sd.Characters[t.params[0]].Level-1, t.params[4]-1) / PurpleBookXP
				mats.mora += int(math.Floor(mats.books / 5.0))
				mats.mora += 20000 * (t.params[5] - 30) / 10
				mats.bossmats += int(math.Floor((float64(t.params[5])-30.0)/10.0*(float64(t.params[5])-30.0)/10.0/2.0)) + int(math.Max(0, float64(t.params[5])-80.0)/5.0)
			}
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

var resinrates = []float64{ //1 resin = x of this
	60000 / 20,                                    //mora
	122500.0 / PurpleBookXP / 20.0,                //xp books
	255.0 / 4000.0,                                //bossmats
	(2.2 + 1.97*3.0 + 0.23*9.0) / 20.0,            //talent books
	(2.2 + 2.4*3.0 + 0.64*9.0 + 0.07*27.0) / 20.0, //weapon mats
	107.0 / 2000.0,                                //artifacts
}

func resinformats(mats materials) (rsn float64) {
	resin := float64(mats.mora) / resinrates[0]
	resin += mats.books / resinrates[1]
	resin += float64(mats.bossmats) / resinrates[2]
	resin += float64(mats.talentbooks) / resinrates[3]
	resin += float64(mats.weaponmats) / resinrates[4]
	return resin
}

//these 3 test functions below should probably go in a diff file
func runLevelTest(t test, config string) (c string) { //params for level test: 0: charid 1: new level 2: new max level
	lines := strings.Split(config, "\n")
	count := 0
	curline := -1
	//fmt.Printf("\n\n%v", config)
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
		end = strings.Index(lines[curline], ";")
	}

	newline := lines[curline][:start+1]
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

type subrolls struct {
	Atk  float64
	AtkP float64
	HP   float64
	HPP  float64
	Def  float64
	DefP float64
	EM   float64
	ER   float64
	CR   float64
	CD   float64
}

func runArtifactTest(t test, config string) (c string) { //params for artifact test: 0: charid
	lines := strings.Split(config, "\n")
	count := 0
	curline := -1
	for count <= t.params[0] { //fixthis
		curline++
		if strings.Contains(lines[curline], "add stats") {
			count++
		}
	}

	newline := lines[curline][0 : strings.Index(lines[curline], "add stats")+9]
	newline += simartiupgrades(getrolls(lines[curline])) + ";"
	lines[curline] = newline

	return strings.Join(lines, "\n")
}

func simartiupgrades(cursubs []float64, msc float64) string {
	avgsubs := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := 0; i < artisims; i++ {
		avgsubs = addsubs(avgsubs, subsubs(addsubs(randomarti(msc), remove8(cursubs)), cursubs))
	}
	for i := range avgsubs {
		avgsubs[i] /= float64(artisims)
	}
	return torolls(addsubs(cursubs, avgsubs))
}

func addsubs(s1, s2 []float64) []float64 {
	add := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range add {
		add[i] = s1[i] + s2[i]
	}
	return add
}

func subsubs(s1, s2 []float64) []float64 {
	sub := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range sub {
		sub[i] = math.Max(0, s1[i]-s2[i])
	}
	return sub
}

var subchance = []int{6, 4, 6, 4, 6, 4, 4, 4, 3, 3}
var srolls = []float64{0.824, 0.941, 1.059, 1.176}

func randomarti(msc float64) []float64 {
	setmult := 0.6 //0.5 for right set, 0.1 for wrong set but offpiece
	arti := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if rand.Float64() > msc*setmult { //the artifact was unusably offset or had the wrong main stat
		return arti
	}
	count := 0
	for count < 4 {
		s := rand.Intn(44)
		ttl := 0
		for i := range subchance {
			ttl += subchance[i]
			if s < ttl {
				s = i
				break
			}
		}
		if arti[s] == 0 {
			count++
			arti[s] += srolls[rand.Intn(4)]
		}
	}

	upgrades := 0
	if rand.Float64() < 0.2 {
		upgrades = -1
	}
	for upgrades < 4 {
		s := rand.Intn(10)
		if arti[s] != 0 {
			upgrades++
			arti[s] += srolls[rand.Intn(4)]
		}
	}
	fmt.Printf("%v", arti)
	return arti
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

func readWepData() []wepjson {
	jsn, err := os.ReadFile("weps.json")
	wjss := make([]wepjson, 0)
	json.Unmarshal(jsn, &wjss)
	//wjs := wepjson{}

	if err != nil {
		fmt.Print("idk halp")
	}
	fmt.Printf("%v", wjss)
	return wjss
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
	data.DPS = data.DPSraw.Mean
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
