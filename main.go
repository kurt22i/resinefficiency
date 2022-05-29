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
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var referencesim = "https://gcsim.app/viewer/share/BGznqjs62S9w8qxpxPu7w" //link to the gcsim that gives rotation, er reqs and optimization priority. actually no er reqs unless user wants, instead, let them use their er and set infinite energy.
//var chars = make([]Character, 4);
var artifarmtime = 126 //how long it should simulate farming artis, set as number of artifacts farmed. 20 resin ~= 1.07 artifacts.
var artifarmsims = 30  //default: -1, which will be 100000/artifarmtime. set it to something else if desired. nvm 30 is fine lol
var domains []string
var simspertest = 100000      //iterations to run gcsim at when testing dps gain from upgrades.
var godatafile = "GOdata.txt" //filename of the GO data that will be used for weapons, current artifacts, and optimization settings besides ER. When go adds ability to optimize for x*output1 + y*output2, the reference sim will be used to determine optimization target.
var good string
var domstring = ""
var optiorder = []string{"ph0", "ph1", "ph2", "ph3"} //the order in which to optimize the characters
var manualOverride = []string{"", "", "", ""}
var optifor = []string{"", "", "", ""} //chars to not optimize artis for
var team = []string{"", "", "", ""}
var dbconfig = ""
var mode6 = 6
var mode85 = false
var optiall = false
var justgenartis = false
var artisonly = false
var guoba []guobadata

func main() {
	flag.IntVar(&simspertest, "i", 10000, "sim iterations per test")
	flag.StringVar(&referencesim, "url", "", "your simulation")
	flag.Parse()

	//noartis = strings.Split(na, ",")

	good2, err2 := os.ReadFile(godatafile)
	good = string(good2)

	if artifarmsims == -1 {
		artifarmsims = 10000 / artifarmtime
	}

	if domstring == "" {
		//refsimdomains
		domains = []string{""}
	} else {
		//domains = strings.Split(domstring, ",")
		//processdomstring()
	}

	time.Now().UnixNano()
	rand.Seed(42)

	//fmt.Printf("here")
	err := run()

	if err != nil || err2 != nil {
		fmt.Printf("Error encountered, ending script: %+v\n%+v", err2, err)
	}

	fmt.Print("\ntesting complete (press enter to exit)")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

type DBData struct {
	Config    string  `json:"config"`
	DPS       float64 `json:"dps"`
	ViewerKey string  `json:"viewer_key"`
}

//https://gcsim.app/viewer/share/perm_fmSSYJ-PJ8oQdyqkMNwV5
func getConfig() string {
	//fetch from: https://viewer.gcsim.workers.dev/gcsimdb
	var data []DBData
	getJson("https://viewer.gcsim.workers.dev/gcsimdb", &data)

	for _, v := range data {
		match := true
		for _, c := range team {
			if !strings.Contains(v.Config, c) {
				match = false
				break
			}
		}
		if match {
			return v.Config
		}
	}

	fmt.Printf("no gcsim db entry found for %v", team)
	return ""
}

var myClient = &http.Client{Timeout: 10 * time.Second}

func getJson(url string, target interface{}) error {
	r, err := myClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

type guobadata struct {
	Hash      string   `json:"hash"`
	DPS       float64  `json:"dps"`
	Artifacts []string `json:"artifacts"`
	Good      string   `json:"-"`
	Config    string   `json:"-"`
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

	if referencesim == "" && dbconfig == "" {
		return errors.New("please input either your team with -team or your simulation with -url!")
	}

	//make a tmp folder if it doesn't exist
	if _, err := os.Stat("./tmp"); !os.IsNotExist(err) {
		//fmt.Println("tmp folder already exists, deleting...")
		// path/to/whatever exists
		os.RemoveAll("./tmp/")
	}
	os.Mkdir("./tmp", 0755)

	err2 := filepath.Walk("./AutoGO/good", func(path string, info fs.FileInfo, err error) error {
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
		var d guobadata
		d.Good = string(file)
		//err = yaml.Unmarshal(file, &d)
		if err != nil {
			return errors.Wrap(err, "")
		}
		d.Hash = info.Name()
		d.Artifacts = []string{"", "", "", ""}
		d.Config = ""

		guoba = append(guoba, d)

		return nil
	})

	if err2 != nil {
		return errors.Wrap(err2, "")
	}

	var data jsondata
	data = readURL(referencesim)
	fmt.Println("running tests...")

	makeOptiOrder(data)

	runArtifactTest()

	for i := range guoba {
		sim := runSim(guobaConfig(i, data.Config))
		guoba[i].DPS = sim.DPS
		fmt.Printf("\n%V done", guoba[i].Hash)
	}

	out, _ := json.Marshal(guoba)
	//os.Remove(data[i].filepath)
	os.WriteFile("results.json", out, 0755)

	return nil
}

func guobaConfig(i int, config string) string {

	lines := strings.Split(config, "\n")
	for i := range lines { //remove all set and stats lines
		if strings.Contains(lines[i], "add stats") { //|| strings.Contains(l, "add set") {
			lines[i] = ""
		} else if strings.Contains(lines[i], "add set") {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n") + "\n" + guoba[i].Config
}

func makeOptiOrder(data jsondata) { //sort the chars by dps in refsim to determine optimization order
	dps := []float64{-1.0, -1.0, -1.0, -1.0}
	for i := range data.CharDPS { //sort the dps's... right now using dps vs primary target, should use their ttl dps
		for j := range dps {
			if data.CharDPS[i].DPS1.Mean > dps[j] {
				for k := 3; k > j; k-- {
					dps[k] = dps[k-1]
				}
				dps[j] = data.CharDPS[i].DPS1.Mean
				break
			}
		}
	}

	for i, c := range data.Characters { //now sort the names accordingly. could probably do both at once with better code
		for j := range dps {
			if data.CharDPS[i].DPS1.Mean == dps[j] {
				optiorder[j] = c.Name
			}
		}
	}
}

func domainid(dom string) int { //returns the internal id for an artifact's domain
	id := -1
	for i, a := range artiabbrs {
		if dom == a {
			id = i
		}
	}

	if id == -1 {
		fmt.Printf("no domain found for %v", dom)
		return -1
	}

	id = (id / 2) * 2 //if it's odd, subtract one
	return id
}

func rarity(wep string) int {
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
	if data.Rarity < 3 {
		data.Rarity = 3
	}
	return data.Rarity
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
	CharDPS   []struct {
		DPS1 FloatResult `json:"1"`
		DPS2 FloatResult `json:"2"`
		DPS3 FloatResult `json:"3"`
	} `json:"damage_by_char_by_targets"`
	DPS float64
}

type FloatResult struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
	SD   float64 `json:"sd"`
}

type wepjson struct {
	Raritys string `json:"rarity"`
	Rarity  int
}

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
		if err2 != nil {
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

func runArtifactTest() { //params for artifact test: 0: domainid, 1: position in domain q
	for i := range optiorder {
		makeNewLines(i)
	}
}

func makeNewLines(c int) {
	runAutoGO(c)
	parseAGresults(c)
}

func runAutoGO(c int) {
	makeTemplate(c)
	cmd := exec.Command("cmd", "/C", "npm run start")
	//cmd := exec.Command("npm run start")
	cmd.Dir = "./AutoGO"
	err := cmd.Run()
	//fmt.Printf("%v", out)
	if err != nil {
		fmt.Printf("%v", err)
	}
	//bufio.NewReader(os.Stdin).ReadBytes('\n') //wait for the user to signify autogo is done by pressing enter
}

func makeTemplate(c int) {
	//exec.Command("node AutoGO/src/createTemplate.js " + GOchars[getCharID(optiorder[c])] + " tmpl").Output()
	cmd := exec.Command("cmd", "/C", "node src/createTemplate.js "+GOchars[getCharID(optiorder[c])]+" tmpl team.json")
	//cmd := exec.Command("npm run start")
	cmd.Dir = "./AutoGO"
	err := cmd.Run()
	//fmt.Printf("%v", out)
	if err != nil {
		fmt.Printf("%v", err)
	}
}

func getCharID(name string) int {
	for i, c := range simChars {
		if c == name {
			return simCharsID[i]
		}
	}
	fmt.Printf("%v not found in list of sim characters", name)
	return -1
}

type AGresult struct {
	User string `json:"user"`
	Data string `json:"data"`
}

func parseAGresults(c int) {
	jsonData, err := os.ReadFile("./AutoGO/output/tmpl.json")
	if err != nil {
		fmt.Printf("%v", err)
	}
	var results []AGresult
	err = json.Unmarshal([]byte(jsonData), &results)
	if err != nil {
		fmt.Printf("%v", err)
	}

	for i := range results { //ttl up all the arti sim iters
		if results[i].User != guoba[i].Hash {
			fmt.Printf("wronguser:%v,%v\n", results[i].User, guoba[i].Hash)
		}
		newsubs := getAGsubs(results[i].Data, results[i].User, i, c)
		guoba[i].Artifacts[c] = results[i].Data
		guoba[i].Config += optiorder[c] + " add stats " + torolls(newsubs) + ";\n"
		//fmt.Printf("\n%v", newsubs)
		//avgsubs = addsubs(avgsubs, newsubs)
	}

	// for i := range avgsubs { //complete the average
	// 	avgsubs[i] /= float64(len(results))
	// 	avgsubs[i] /= float64(ispct[i])
	// }

}

type GOarti struct {
	SetKey      string `json:"setKey"`
	Rarity      int    `json:"rarity"`
	Level       int    `json:"level"`
	SlotKey     string `json:"slotKey"`
	MainStatKey string `json:"mainStatKey"`
	Substats    []struct {
		Key   string  `json:"key"`
		Value float64 `json:"value"`
	} `json:"substats"`
	Location string `json:"location"`
	Exclude  bool   `json:"exclude"`
	Lock     bool   `json:"lock"`
}

func deleteArtis(file string, artistats []float64) {
	f, err := os.ReadFile("./AutoGO/good/" + file)
	if err != nil {
		fmt.Printf("%v", err)
	}
	rawgood := string(f)
	artisection := ""
	if strings.Index(rawgood, "weapons\"") == -1 {
		if strings.LastIndex(rawgood, "exclude\"") == -1 {
			//fmt.Printf("whyyyyy\nwhyyy\n%v", rawgood)
			//artisection = "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.Index(rawgood, "}]}")+4]
			artisection = "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.LastIndex(rawgood, "}]}")+2]
		} else {
			artisection = "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.LastIndex(rawgood, "exclude\"")+strings.LastIndex(rawgood[strings.LastIndex(rawgood, "exclude"):], "}]")+2]
		}
	} else if strings.Index(rawgood, "weapons\"") < 100 {
		artisection = "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.Index(rawgood, "characters\"")-2]
	} else {
		//fmt.Printf("whyyyyy\nwhyyy\n%v", rawgood)
		artisection = "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.Index(rawgood, "weapons\"")-2]
	}

	var artis []GOarti
	err = json.Unmarshal([]byte(artisection), &artis)
	//asnowman := subsubs(ar)
	found := false
	for i := range artis { //this currently works by looking for an arti with 3 stats = and 1 stat bigger (main stat), should be good enough?
		toobig := 0
		for j, s := range artis[i].Substats {
			if s.Key == "" || artistats[getStatID(s.Key)] < s.Value {
				break
			} else if artistats[getStatID(s.Key)] > s.Value {
				toobig++
				if toobig >= 2 {
					break
				}
			}
			if j == 3 {
				found = true
				//fmt.Printf("\n%v", artis[i])
				artis[i] = artis[len(artis)-1]
				artis = artis[:len(artis)-1]
			}
		}
		if found {
			break
		}
	}

	if !found {
		fmt.Printf("failed to delete artifact %v,%v,%v,%v", artistats, err, artisection, rawgood)
	}

	marsh, err := json.Marshal(artis)
	if err != nil {
		fmt.Printf("%v", err)
	}
	newas := string(marsh)
	newas = newas[1:len(newas)-1] + "]"
	newgood := ""
	if strings.Index(rawgood, "weapons\"") == -1 {
		newgood = rawgood[:strings.Index(rawgood, "artifacts\"")+12] + newas + "}"
	} else if strings.Index(rawgood, "weapons\"") < 100 {
		newgood = rawgood[:strings.Index(rawgood, "artifacts\"")+12] + newas + rawgood[strings.Index(rawgood, "characters\"")-2:]
	} else {
		newgood = rawgood[:strings.Index(rawgood, "artifacts\"")+12] + newas + rawgood[strings.Index(rawgood, "weapons\"")-2:]
	}
	err = os.WriteFile("./AutoGO/good/"+file, []byte(newgood), 0755)
	if err != nil {
		fmt.Printf("%v", err)
	}
}

func getAGsubs(raw, file string, id, c int) []float64 {
	subs := newsubs()
	//fmt.Printf("%v", raw)
	artis := strings.Split(raw, "|")
	sets := []string{"", "", "", "", ""}
	setcounts := []int{0, 0, 0, 0, 0}
	for _, a := range artis {
		if a == "" {
			continue
		}
		asubs := newsubs()
		set := a[:strings.Index(a, ":")]
		found := false
		curset := 0
		for !found {
			if sets[curset] == set {
				found = true
				setcounts[curset]++
			} else if sets[curset] == "" {
				sets[curset] = set
				setcounts[curset]++
				found = true
			} else {
				curset++
			}
		}
		stats := strings.Split(a[strings.Index(a, ":")+1:], "~")
		for _, s := range stats {
			if s == "" {
				continue
			}
			stattype := s[:strings.Index(s, "=")]
			val := s[strings.Index(s, "=")+1:]
			ispt := false
			if strings.Contains(val, "%") {
				val = val[:len(val)-1]
				ispt = true
			}
			parse, err := strconv.ParseFloat(val, 64)
			if err != nil {
				fmt.Printf("%v", err)
			}
			asubs[AGstatid(stattype, ispt)] += parse
		}
		deleteArtis(file, asubs) //delete the artis chosen so that they're not selected again for another char
		subs = addsubs(subs, asubs)
	}

	for i := range subs {
		subs[i] /= float64(ispct[i])
	}

	for i := range sets {
		if setcounts[i] >= 2 {
			guoba[id].Config += optiorder[c] + " add set=\"" + fixname(sets[i]) + "\" count=" + strconv.Itoa(setcounts[i]) + ";\n"
		}
	}

	return subs
}

func fixname(str string) string {
	s := str
	s = strings.Replace(s, "'", "", -1)
	s = strings.Replace(s, " ", "", -1)
	// for strings.Index(s," ") > 0 {
	// 	s = s[:strings.Index(s," ")] + s[strings.Index(s, " ")+1:]
	// }
	return strings.ToLower(s)
}

func newsubs() []float64 { //empty stat array
	return []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

func AGstatid(key string, ispt bool) int {
	for i, k := range AGstatKeys {
		if k == key {
			if i < 6 && ispt {
				return i + 1 //the key for flat vs % hp, atk and def is the same, so we have to look at the value
			}
			return i
		}
	}
	fmt.Printf("no stat found for the AG key %v", key)
	return -1
}

func getStatID(key string) int {
	for i, k := range statKey {
		if k == key {
			return i
		}
	}
	fmt.Printf("%v not recognized as a key", key)
	return -1
}

//ugly sorting code - sorts sim chars by dps, which is the order we should optimize them in (except this is a waste bc user has to specify anyway rn)
/*chars := []string{"", "", "", ""}
chardps := []float64{-1.0, -1.0, -1.0, -1.0}
for i := range baseline.CharDPS {
	chardps[i] = baseline.CharDPS[i].DPS1.Mean
}
sort.Float64s(chardps)
for i := range baseline.Characters {
	for j := range chardps {
		if baseline.CharDPS[i].DPS1.Mean == chardps[j] {
			chars[j] = baseline.Characters[i].Name
		}
	}
}*/

func farmJSONs(domain int) {
	files, err := filepath.Glob("./AutoGO/good/gojson*")

	if err != nil {
		fmt.Printf("%v", err)
	}
	for _, f := range files {
		os.Remove(f)
	}
	for j := 0; j < artifarmsims; j++ {
		artistartpos := strings.Index(good, "artifacts\"") + 12
		newartis := ""
		for i := 0; i < artifarmtime; i++ {
			newartis += randomGOarti(domain)
		}
		//fmt.Printf("%v", newartis)
		gojsondata := good[:artistartpos] + newartis + good[artistartpos:]

		//fmt.Printf("%v", gojsondata)
		//fmt.Printf("./" + fmt.Sprintf("%0.f", float64(domain)) + "/gojson" + fmt.Sprintf("%.0f", float64(t)) + ".txt")
		//os.WriteFile("./"+fmt.Sprintf("%0.f", float64(domain))+"/gojson"+fmt.Sprintf("%.0f", float64(t))+".txt", []byte(gojsondata), 0755)
		os.WriteFile("./AutoGO/good/gojson"+fmt.Sprintf("%0.f", float64(j))+".json", []byte(gojsondata), 0755) //int->float->int shouldnt be necessary lol
	}
}

var artinames = []string{"BlizzardStrayer", "HeartOfDepth", "ViridescentVenerer", "MaidenBeloved", "TenacityOfTheMillelith", "PaleFlame", "HuskOfOpulentDreams", "OceanHuedClam", "ThunderingFury", "Thundersoother", "EmblemOfSeveredFate", "ShimenawasReminiscence", "NoblesseOblige", "BloodstainedChivalry", "CrimsonWitchOfFlames", "Lavawalker"}
var artiabbrs = []string{"bs", "hod", "vv", "mb", "tom", "pf", "husk", "ohc", "tf", "ts", "esf", "sr", "no", "bsc", "cw", "lw"}

var simChars = []string{"ganyu", "rosaria", "kokomi", "venti", "ayaka", "mona", "albedo", "fischl", "zhongli", "raiden", "bennett", "xiangling", "xingqiu", "shenhe", "yae", "kazuha", "beidou", "sucrose", "jean", "chongyun", "yanfei", "keqing", "tartaglia", "eula", "lisa", "yunjin"}
var simCharsID = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}
var GOchars = []string{"Ganyu", "Rosaria", "SangonomiyaKokomi", "Venti", "KamisatoAyaka", "Mona", "Albedo", "Fischl", "Zhongli", "RaidenShogun", "Bennett", "Xiangling", "Xingqiu", "Shenhe", "YaeMiko", "KaedeharaKazuha", "Beidou", "Sucrose", "Jean", "Chongyun", "Yanfei", "Keqing", "Tartaglia", "Eula", "Lisa", "YunJin"}

var slotKey = []string{"flower", "plume", "sands", "goblet", "circlet"}
var statKey = []string{"atk", "atk_", "hp", "hp_", "def", "def_", "eleMas", "enerRech_", "critRate_", "critDMG_", "heal_", "pyro_dmg_", "electro_dmg_", "cryo_dmg_", "hydro_dmg_", "anemo_dmg_", "geo_dmg_", "physical_dmg_"}
var AGstatKeys = []string{"Atk", "n/a", "hp", "n/a", "Def", "n/a", "ele_mas", "EnergyRecharge", "CritRate", "CritDMG", "HealingBonus", "pyro", "electro", "cryo", "hydro", "anemo", "geo", "physicalDmgBonus"}
var msv = []float64{311.0, 0.466, 4780, 0.466, -1, 0.583, 187, 0.518, 0.311, 0.622, 0.359, 0.466, 0.466, 0.466, 0.466, 0.466, 0.466, 0.583} //def% heal and phys might be wrong
var ispct = []int{1, 100, 1, 100, 1, 100, 1, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}

func gcsimArtiName(abbr string) string {
	for i, a := range artiabbrs {
		if a == abbr {
			return strings.ToLower(artinames[i])
		}
	}
	fmt.Printf("arti abbreviation %v not recognized", abbr)
	return ""
}

func randomGOarti(domain int) string {
	arti := "{\"setKey\":\""
	//if rand.Intn(2) == 0 {
	//	arti += "MaidenBeloved"
	//} else {
	arti += artinames[domain+rand.Intn(2)]
	//}
	arti += "\",\"rarity\":5,\"level\":20,\"slotKey\":\""
	artistats := randomarti()
	arti += slotKey[int(artistats[10])]
	arti += "\",\"mainStatKey\":\""
	arti += statKey[int(artistats[11])]
	arti += "\",\"substats\":["
	curpos := 0
	found := 0
	for found < 4 {
		if artistats[curpos] > 0 {
			arti += "{\"key\":\""
			arti += statKey[curpos]
			arti += "\",\"value\":"
			if ispct[curpos] == 1 {
				arti += fmt.Sprintf("%.0f", standards[curpos]*artistats[curpos])
			} else {
				arti += fmt.Sprintf("%.1f", 100.0*standards[curpos]*artistats[curpos])
			}
			arti += "}"
			if found < 3 {
				arti += ","
			}
			found++
		}
		curpos++
	}
	arti += "],\"location\":\"\",\"exclude\":false,\"lock\":true},"
	return arti
}

var standards = []float64{16.54, 0.0496, 253.94, 0.0496, 19.68, 0.062, 19.82, 0.0551, 0.0331, 0.0662}

func torolls(subs []float64) string {
	str := "atk=" + fmt.Sprintf("%f", subs[0])
	str += " atk%=" + fmt.Sprintf("%f", subs[1])
	str += " hp=" + fmt.Sprintf("%f", subs[2])
	str += " hp%=" + fmt.Sprintf("%f", subs[3])
	str += " def=" + fmt.Sprintf("%f", subs[4])
	str += " def%=" + fmt.Sprintf("%f", subs[5])
	str += " em=" + fmt.Sprintf("%f", subs[6])
	str += " er=" + fmt.Sprintf("%f", subs[7])
	str += " cr=" + fmt.Sprintf("%f", subs[8])
	str += " cd=" + fmt.Sprintf("%f", subs[9])
	str += " heal=" + fmt.Sprintf("%f", subs[10])
	str += " pyro%=" + fmt.Sprintf("%f", subs[11])
	str += " electro%=" + fmt.Sprintf("%f", subs[12])
	str += " cryo%=" + fmt.Sprintf("%f", subs[13])
	str += " hydro%=" + fmt.Sprintf("%f", subs[14])
	str += " anemo%=" + fmt.Sprintf("%f", subs[15])
	str += " geo%=" + fmt.Sprintf("%f", subs[16])
	str += " phys%=" + fmt.Sprintf("%f", subs[17])
	return str
}

func addsubs(s1, s2 []float64) []float64 {
	add := newsubs()
	for i := range add {
		add[i] = s1[i] + s2[i]
	}
	return add
}

func subsubs(s1, s2 []float64) []float64 {
	sub := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range sub {
		sub[i] = s1[i] - s2[i] //math.Max(0, s1[i]-s2[i])
	}
	return sub
}

func multsubs(s []float64, mult float64) []float64 {
	sub := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range sub {
		sub[i] = s[i] * mult
	}
	return sub
}

var subchance = []int{6, 4, 6, 4, 6, 4, 4, 4, 3, 3}
var srolls = []float64{0.824, 0.941, 1.059, 1.176}

/*type subrolls struct {
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
}*/ //then: heal, pyro,electro,cryo,hydro,anemo,geo,phys

var rollints = []int{1, 1, 30, 80, 50}
var mschance = [][]int{ //chance of mainstat based on arti type
	{0, 0, 1},
	{1},
	{0, 8, 0, 8, 0, 8, 3, 3},
	{0, 17, 0, 17, 0, 16, 2, 0, 0, 0, 0, 4, 4, 4, 4, 4, 4, 4},
	{0, 11, 0, 11, 0, 11, 2, 0, 5, 5, 5},
}

func randomarti() []float64 {
	arti := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	arti[10] = float64(rand.Intn(5)) //this is type, 0=flower, 1=feather, etc... all these type conversions can't be ideal, should do this a diff way
	m := rand.Intn(rollints[int(arti[10])])
	ttl := 0
	for i := range mschance[int(arti[10])] {
		ttl += mschance[int(arti[10])][i]
		if m < ttl {
			arti[11] = float64(i)
			break
		}
	}

	count := 0
	for count < 4 {
		s := rand.Intn(44)
		ttl = 0
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

	//arti[7] = 0 //no er allowed

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
	if err != nil {
		fmt.Printf("error reading db sim: %v\n", err)
	}
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
