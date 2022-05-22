package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
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
var artis []Artifact

func main() {
	flag.IntVar(&simspertest, "i", 10000, "sim iterations per test")
	flag.Parse()

	readArtifacts()
	calcRV()
	printResults()
}

type Artifact struct {
	Set      string
	Substats []float64
	Lines    int
	Mainstat int
	Level    int
	Slot     int
	BestOn   int
	RVon     int
	BestOff  int
	RVoff    int
	Rarity   int
}

func getSetID(dom string) int { //returns the internal id for an artifact
	id := -1
	for i, a := range artinames {
		if dom == a {
			id = i
		}
	}

	if id == -1 {
		fmt.Printf("no set found for %v", dom)
		return -1
	}

	return id
}

func printResults(res, base result) {
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

func readArtifacts() {
	f, err := os.ReadFile(godatafile)
	if err != nil {
		fmt.Printf("%v", err)
	}
	rawgood := string(f)
	artisection := "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.Index(rawgood, "weapons\"")-2]

	var gartis []GOarti
	err = json.Unmarshal([]byte(artisection), &artis)
	//asnowman := subsubs(ar)
	for i := range gartis { //this currently works by looking for an arti with 3 stats = and 1 stat bigger (main stat), should be good enough?
		var art Artifact
		art.Set = artiabbrs[getSetID(gartis[i].SetKey)]
		art.Lines = 0
		art.Mainstat = getStatID(gartis[i].MainStatKey)
		art.Level = gartis[i].Level
		art.Rarity = gartis[i].Rarity
		art.Slot = getSlotID(gartis[i].SlotKey)
		art.Substats = newsubs()
		art.BestOn = -1
		art.BestOff = -1
		art.RVon = -1
		art.RVoff = -1
		for j, s := range gartis[i].Substats {
			if s.Key == "" {
				break
			}

		}
		artis = append(artis, art)
	}
}

func getAGsubs(raw, file string) []float64 {
	subs := newsubs()
	//fmt.Printf("%v", raw)
	artis := strings.Split(raw, "|")
	for _, a := range artis {
		if a == "" {
			continue
		}
		asubs := newsubs()
		stats := strings.Split(a, "~")
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
	return subs
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

func getSlotID(key string) int {
	for i, k := range slotKey {
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
