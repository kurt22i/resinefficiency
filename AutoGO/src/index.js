const puppeteer = require("puppeteer")
const { readFile, readdir, writeFile, mkdir } = require("fs/promises")
const { join } = require("path")
const settings = require("../settings.json")

async function run() {
    const browser = await puppeteer.launch({ headless: false })
    await mkdir("output", { recursive: true })


    console.log()
    console.log(`Running builds for ${settings.onlyNew ? "only new users" : "all users"} for ${settings.templates.length} template(s)`)
    console.log("=".repeat(64))

    for (const templateFile of settings.templates) {
        const { templateName, template, char } = JSON.parse((await readFile(templateFile)).toString())

        console.log()
        console.log(`Starting template ${templateName}`)

        const url = `https://frzyc.github.io/genshin-optimizer/#/character/${char}/build`
        const outputFile = `output/${templateName}.json`
        const output = await loadOutput(outputFile)

        if (output.length > 0)
            console.log(`Loaded ${output.length} from output`)
        console.log("=".repeat(64))

        for (const f of await readdir("./good/", { withFileTypes: true }))
            if (f.isFile() && f.name.endsWith(".json")) {
                const { name: user } = f
                if (settings.onlyNew && output.some(x => x.user == user))
                    continue

                const good = await prepareUser(template, user, templateName)

                const page = await browser.newPage()
                console.log(`Replacing database for ${templateName}/${user}`)
                await page.goto("https://frzyc.github.io/genshin-optimizer/#/setting")
                await page.waitForSelector("textarea")
                await page.evaluate(`document.querySelector("textarea").value = \`${JSON.stringify(good).replace(/[\\`$]/g, "\\$&")}\`;`)
                await page.type("textarea", " ")
                await page.waitForTimeout(500)
                await clickButton(page, "Replace Database")
                await page.waitForTimeout(500)

                console.log(`Starting build generation for ${templateName}/${user}`)
                await page.goto(url)
                await page.waitForTimeout(1000)
                //const buildbutton = await page.$("#root > div.MuiGrid-root.MuiGrid-container.MuiGrid-direction-xs-column.css-14bwzpa > div.MuiContainer-root.MuiContainer-maxWidthXl.css-11xxdke > div > div > div.MuiCardContent-root.css-182b5p1 > div:nth-child(2) > div > div.MuiTabs-scroller.MuiTabs-hideScrollbar.MuiTabs-scrollableX.css-12qnib > div > button:nth-child(5)")
               // await buildbutton.click()
                await clickButton(page, "Generate")

                await busyWait(page, user)
                
                console.log(`Exporting data of ${templateName}/${user}`)
                await page.waitForTimeout(500)

                var str = ""
                for (let i=0;i<5;i++) {//each artifact
//document.querySelector("")
                    const artitype = await page.$(`#root > div.MuiGrid-root.MuiGrid-container.MuiGrid-direction-xs-column.css-14bwzpa > div.MuiContainer-root.MuiContainer-maxWidthXl.css-11xxdke > div > div > div > div.MuiBox-root.css-1gv0prw > div:nth-child(6) > div > div.MuiGrid-root.MuiGrid-container.MuiGrid-spacing-xs-1.css-eujw0i > div:nth-child(${i+2}) > div > button > div > div.grad-5star.MuiBox-root.css-1fm09i6 > h6`)
                    var raw = await (await artitype.getProperty("innerHTML")).jsonValue()
                    raw = raw.substring(raw.indexOf("icon=")+6)
                    str += raw.substring(0, raw.indexOf("\"")) + "="
                    str += await (await artitype.getProperty("innerText")).jsonValue() + "~"
                    for (let j=0;j<4;j++) {//substats
                        const substat = await page.$(`#root > div.MuiGrid-root.MuiGrid-container.MuiGrid-direction-xs-column.css-14bwzpa > div.MuiContainer-root.MuiContainer-maxWidthXl.css-11xxdke > div > div > div > div.MuiBox-root.css-1gv0prw > div:nth-child(6) > div > div.MuiGrid-root.MuiGrid-container.MuiGrid-spacing-xs-1.css-eujw0i > div:nth-child(${i+2}) > div > button > div > div.MuiBox-root.css-11yya3r > div:nth-child(${j+1})`)
                        var raw = await (await substat.getProperty("innerHTML")).jsonValue()
                        raw = raw.substring(raw.indexOf("icon=")+6)
                        str += raw.substring(0, raw.indexOf("\"")) + "="
                        str += await (await substat.getProperty("innerText")).jsonValue() + "~"
                    }
                    str += "|"
                }
                console.log(str)

                output.push({
                    user,
                    data: str
                })
                await writeFile(outputFile, JSON.stringify(output))

                await page.close()
            }
    }
    await browser.close()
}

/**
 * @typedef Output
 * @property {string} name
 * @property {number[][]} stats
 */

/**
 * 
 * @param {string} file Path of file to load
 * @returns {Promise<Output[]>} Currently loaded output
 */
async function loadOutput(file) {
    if (!settings.onlyNew)
        return []

    let contents
    try {
        contents = await readFile(file)
    } catch (error) {
        return []
    }

    return JSON.parse(contents.toString())
}

/**
 * Prepare user data, filling in a template
 * @param {GOOD} template Template data to fill in
 * @param {string} user Name of user
 * @param {string} templateName Name of template
 * @returns {Promise<GOOD>} Filled in GOOD data
 */
async function prepareUser(template, user, templateName) {
    console.log(`Preparing data for ${templateName}/${user}`)
    const userGood = JSON.parse((await readFile(join("good", user))).toString())
    const good = Object.assign({}, template, { artifacts: userGood.artifacts })

    // Clean up artifact settings
    good.artifacts = good.artifacts.map(a => Object.assign(a, {
        "location": "",
        "exclude": false,
        "lock": false
    }))

    // Enable TC mode
    good.states = [{
        "tcMode": true,
        "key": "GlobalSettings"
    }]

    return good
}

/**
 * Click a button element with a certain text
 * @param {puppeteer.Page} page The current tab
 * @param {string} targetText Text of the button to press
 * @returns 
 */
async function clickButton(page, targetText) {
    const buttons = await page.$$("button")

    for (const button of buttons) {
        const text = await (await button.getProperty("innerText")).jsonValue()
        if (text == targetText) {
            await button.click()
            return
        }
    }
    console.error(`Could not find button with name ${targetText}`)
}


/**
 * Busily wait for build generation to finish, prints progress ever ~3 seconds
 * @param {puppeteer.Page} page The current tab
 * @param {string} user Name of the current user
 * @returns when build generation is done
 */
async function busyWait(page, user) {
    while (true) {
        await page.waitForTimeout(3000)
        const message = await page.$(".MuiAlert-message")
        const text = await (await message.getProperty("innerText")).jsonValue()
        console.log(`${user}: ${text.replace(/\n+/g, " / ")}`)

        if (text.startsWith("Generated")) return
    }
}

run()