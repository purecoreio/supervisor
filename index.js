const { Supervisor, ConsoleUtil } = require("./target/supervisor");
const supervisor = new Supervisor(null);

ConsoleUtil.showTitle();

supervisor.emitter.on("loadingMachine", () => {
    ConsoleUtil.setLoading(true, "Loading machine")
}).on("gotMachine", () => {
    ConsoleUtil.setLoading(false, "Got machine", false)
}).on("errorGettingMachine", (err) => {
    ConsoleUtil.setLoading(false, "Error while loading the machine: " + err.message, true)
    ConsoleUtil.askHash().then((hash) => {
        supervisor.setup(hash);
    })
}).on("hashSavingError", () => {
    ConsoleUtil.setLoading(false, "Error while saving the hash. Please, run this tool as 'sudo'", true)
}).on("hashLoadingError", () => {
    ConsoleUtil.setLoading(false, "Error while reading the hash. Please, run this tool as 'sudo'", true)
}).on("pushingHardware", () => {
    ConsoleUtil.setLoading(true, "Pushing hardware components")
}).on("pushedHardware", () => {
    ConsoleUtil.setLoading(false, "Pushed hardware components", false)
}).on("errorPushingHardware", () => {
    ConsoleUtil.setLoading(false, "Error while pushing hardware components", true)
}).on("pushingNetwork", () => {
    ConsoleUtil.setLoading(true, "Pushing network adapters")
}).on("pushedNetwork", () => {
    ConsoleUtil.setLoading(false, "Pushed network adapters", false)
}).on("errorPushingNetwork", () => {
    ConsoleUtil.setLoading(false, "Error while pushing network adapters", true)
}).on("checkingCorrelativity", () => {
    ConsoleUtil.setLoading(true, "Checking docker's correlativity with the existing filesystem")
}).on("checkedCorrelativity", () => {
    ConsoleUtil.setLoading(false, "Checked docker's correlativity with the existing filesystem", false)
}).on("errorCheckingCorrelativity", () => {
    ConsoleUtil.setLoading(false, "Error while checking docker's correlativity with the existing filesystem. Is docker active? Execute this service as 'sudo'", true)
})

supervisor.setup();