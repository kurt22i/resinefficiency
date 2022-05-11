# resinefficiency

this tool calculates the most resin efficient upgrades for your team by calculating the DPS increase/resin cost of each upgrade, including farming artifacts. It aims to help answer questions like "what's the best way to improve my team?" and "am i really gaining anything from farming this domain, or is it time to stop?". I don't really know where I'm going with this project yet and it might continue to be developed or it might not, idk. I'm not really a programmer and the code is a huge mess, which makes it hard to add new features. If you want to lose your sanity, PRs are welcome.

# How to Use

1. Download and unzip the code
2. Open cmd prompt and navigate to the resinefficiency-main/AutoGO directory
3. Run the command ```npm i``` to install dependencies
4. In the main resin efficiency folder, open the GOdata.txt file and paste in your own GO database
5. In the main folder, run the command ```go run . -team="char1,char2,char3,char4"```
6. Press Enter and watch the calc! At default iterations of 10000, it should take about 2 minutes. (artifacts are not tested by default - they can be enabled with the -d parameter below)

Options:</br>
-d (string) which artifact domains to farm. Example format: ```-d="bs(ayaka&ganyu),vv(venti)"```</br>
-i (int) number of iterations per test</br>
-ft (int) number of artifacts to farm</br>
-onlyartis (bool) only run artifact tests</br>

contact Kurt#5846 with questions/suggestions/bugs/etc!

credits:
- srl#2712: codebase, answering my numerous dumb go questions
- Shizuka#7791: answering my numerous dumb go questions
- theBowja/genshin-db: jsons for the weapons
- all the gcsim devs and contributors
- frzyc#3029 and all the GO devs
- Tibo#4309 for writing and letting me use AutoGO
