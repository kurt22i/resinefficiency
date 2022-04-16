# resinefficiency
 
this tool calculates the most resin efficient upgrades for your team by calculating the DPS increase/resin cost of each upgrade. It's currently in pre-pre-alpha so there might be bugs (it will probably stay like this for quite a while, as I really don't have the coding knowledge to expand it much farther. PRs welcome though if you have ideas or just want to clean up my messy code)

# How to Use

1. Click Code -> Download ZIP
2. In your downloads folder, right click the ZIP, and click Extract All
3. Navigate inside the "resinefficiency-main" folders
4. In the address bar (the bar that says something like "Downloads > resinefficiency-main > resinefficiencymain"), type "cmd" and press Enter
5. Paste in the following command: go run . -url=linkhere
6. Replace linkhere with the link to your personal sim
7. Press Enter and watch the calc! At default iterations of 10000, it should take about 2 minutes.

Additional options:</br>
-i (int) number of iterations per test</br>
-halp (bool) if you're getting a zlib error try adding this, the error happens when the linked sim was created on desktop rather than web</br>

contact Kurt#5846 with questions/suggestions/bugs/etc!

credits:
- srl#2712: codebase, answering my numerous dumb go questions
- Shizuka#7791: answering my numerous dumb go questions
- theBowja/genshin-db: jsons for the weapons
- all the gcsim contributors
