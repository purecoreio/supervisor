var Benchmark = require('benchmark');
var suite = new Benchmark.Suite;
var colors = require('colors');
const inquirer = require('inquirer')
global.fetch = require("node-fetch");
const si = require('systeminformation');
const server = require('http').createServer();
const io = require('socket.io')(server);

const Core = require('purecore')

var character = ""
var loadingInterval
var loadingStep

var nativeMachine;

const publicIp = require('public-ip');
var ipv4 = null;

function createServerListener() {


    setLoading(true, "Updating IPV4")
    publicIp.v4().then(function (ip) {
        setLoading(false, "Updated TCP info")
        nativeMachine.setIPV4(ip);
        ipv4 = ip;

        setLoading(true, "Creating socket server")

        io.on('connection', client => {
            console.log("Connected: " + client.id)
            client.on('getTemperature', data => {
                si.cpuTemperature().then(function (cpuTemperature) {
                    client.emit("temperature", cpuTemperature)
                })
            });
            client.on('disconnect', () => { /* … */ });
        });

        server.listen(31518);
        setLoading(false, "Created socket server @ " + ipv4 + ":31518")

    })

    publicIp.v6().then(function (ip) {
        nativeMachine.setIPV6(ip);
    })

}

function setLoading(status, string, failed = false) {

    if (status) {

        loadingInterval = setInterval(() => {

            loadingStep++

            process.stdout.clearLine()
            process.stdout.cursorTo(0)

            switch (loadingStep) {
                case 1:
                    character = " ▖  "
                    break;
                case 2:
                    character = " ▘  "
                    break;
                case 3:
                    character = " ▝  "
                    break;
                default:
                    character = " ▗  "
                    loadingStep = 0
                    break;
            }

            process.stdout.write(character.bgRed + " " + string.red)

        }, 100);
    } else {

        clearInterval(loadingInterval);
        process.stdout.clearLine()
        process.stdout.cursorTo(0)

        if (!failed) {
            process.stdout.write(" ✓  ".bgGreen + " " + string.white)
        } else {
            process.stdout.write(" ☓  ".bgRed + " " + string.white)
        }

        process.stdout.write("\n");
        process.stdout.cursorTo(0)

    }

}

function runBenchmark() {

    setLoading(true, "Performing benchmark")

    suite.add('String#indexOf', function () { 'Hello World!'.indexOf('o') > -1; }).on('complete', function (data) {
        setLoading(false, String("Benchmark result relative to quiquelhappy: " + String(Math.floor(data.target.hz) / 989415652)))
        setLoading(false, String("Benchmark count relative to quiquelhappy: " + String(data.target.count - 50057820)))
    }).run({ 'async': true });


}

function showTitle() {

    console.log("                                                         _                ".white)
    console.log("                                                        (_)               ".white)
    console.log("   ___ ___  _ __ ___     ___ _   _ _ __   ___ _ ____   ___ ___  ___  _ __ ".white)
    console.log("  / __/ _ \\| '__/ _ \\   / __| | | | '_ \\ / _ \\ '__\\ \\ / / / __|/ _ \\| '__|".white)
    console.log(" | (_| (_) | | |  __/_  \\__ \\ |_| | |_) |  __/ |   \\ V /| \\__ \\ (_) | |   ".white)
    console.log("  \\___\\___/|_|  \\___(_) |___/\\__,_| .__/ \\___|_|    \\_/ |_|___/\\___/|_|   ".white)
    console.log("                                  | |                                     ".white)
    console.log("                                  |_|                                     ".white)
    console.log("")
    console.log("     ◢ by © quiquelhappy ".white)
}

showTitle()

var settings
const fs = require('fs');

try {

    setLoading(true, "Loading settings")

    let rawdata = fs.readFileSync('./settings.json');
    settings = JSON.parse(rawdata);

    if (!("hash" in settings)) {

        throw new Error("invalid settings")

    } else {
        setLoading(false, "Settings loaded")
        finishStartup()
    }

} catch (error) {

    setLoading(false, "Error while loading settings", true)

    setLoading(true, "Regenerating settings file")

    if (fs.existsSync("./settings.json")) {

        fs.unlinkSync('./settings.json', function (data) {

            setLoading(false, "Error while regenerating settings", true)

        })

        fs.writeFile('./settings.json', "{ \"hash\":null}", function (data) {
            setLoading(false, "Regenerated settings file")
            finishStartup()
        })

    } else {
        fs.writeFile('./settings.json', "{ \"hash\":null}", function (data) {

            setLoading(false, "Generated settings file")
            finishStartup()

        })
    }

    settings = { "hash": null }

}

function pushSystem() {
    var core = new Core();

    setLoading(true, "Loading machine definition")

    core.getMachine(settings.hash).then(function (machine) {

        setLoading(false, "Loaded machine from the cloud")
        setLoading(true, "Getting machine components")

        si.getStaticData(function (components) {
            setLoading(false, "Got machine components")
            setLoading(true, "Uploading components")
            machine.updateComponents(components).then(function (machine) {
                setLoading(false, "Uploaded components")

                nativeMachine = machine;

                createServerListener()

            }).catch(function (error) {
                setLoading(false, "Error while uploading components")
            })
        })

    }).catch(function (error) {
        setLoading(false, error.message, true)
        settings.hash = null;
        finishStartup();
    })
}

function finishStartup() {

    var questions = [{
        type: 'password',
        name: 'hash',
        prefix: " ?  ".bgGreen,
        message: "Please, enter your machine hash and press enter",
    }]

    if (settings.hash == null) {
        inquirer.prompt(questions).then(answers => {

            settings.hash = answers['hash'];

            let data = JSON.stringify(settings);
            fs.writeFileSync('./settings.json', data);
            pushSystem();

        })

    } else {
        pushSystem();
    }

}
