var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
const core = require('purecore');
const EventEmitter = require('events');
class Supervisor {
    constructor(hash) {
        // events
        this.emitter = new EventEmitter();
        if (Supervisor.hash != hash && hash != null) {
            MachineSettings.getHash().then((hash) => {
                Supervisor.hash = hash;
            });
        }
    }
    setup(hash) {
        return __awaiter(this, void 0, void 0, function* () {
            let main = this;
            if (hash != null) {
                yield MachineSettings.setHash(hash).catch(() => {
                    main.emitter.emit('hashSavingError');
                });
                Supervisor.hash = hash;
            }
            else {
                yield MachineSettings.getHash().then((storedHash) => {
                    Supervisor.hash = storedHash;
                }).catch(() => {
                    main.emitter.emit('hashLoadingError');
                });
            }
            main.emitter.emit('loadingMachine');
            new core().getMachine(Supervisor.hash).then((machine) => {
                main.emitter.emit('gotMachine');
                Supervisor.machine = machine;
                main.IOCheck().then(() => {
                    main.emitter.emit('checkingCorrelativity');
                    Correlativity.updateFolders().then(() => {
                        main.emitter.emit('checkedCorrelativity');
                    }).catch(() => {
                        main.emitter.emit('errorCheckingCorrelativity');
                    });
                }).catch((err) => {
                    // can't complete the setup process
                });
            }).catch((err) => {
                main.emitter.emit('errorGettingMachine', err);
            });
        });
    }
    static getSupervisor() {
        return Supervisor;
    }
    static getMachine() {
        return Supervisor.machine;
    }
    IOCheck() {
        let main = this;
        return new Promise(function (resolve, reject) {
            main.emitter.emit('pushingHardware');
            HardwareCheck.updateComponents().then(() => {
                main.emitter.emit('pushedHardware');
                main.emitter.emit('pushingNetwork');
                NetworkCheck.updateNetwork().then(() => {
                    main.emitter.emit('pushedNetwork');
                    resolve();
                }).catch((err) => {
                    main.emitter.emit('errorPushingNetwork');
                    reject(err);
                });
            }).catch((err) => {
                main.emitter.emit('errorPushingHardware');
                reject(err);
            });
        });
    }
}
// actual props
Supervisor.hash = null;
Supervisor.ready = false;
module.exports.Supervisor = Supervisor;
const dockerode = require('dockerode');
const fs = require('fs');
class Correlativity {
    /**
     * Will move the folders on /opt/purecore/hosted/ to /opt/purecore/tmp/ when no purecore.io docker is matching that data
     */
    static updateFolders() {
        return new Promise(function (resolve, reject) {
            try {
                var docker = new dockerode();
                docker.listContainers(function (err, containers) {
                    let existingContainers = [];
                    if (containers == null) {
                        reject();
                    }
                    else {
                        containers.forEach(function (containerInfo) {
                            for (var name of docker.getContainer(containerInfo.Id).Names) {
                                name = String(name);
                                const prefix = "core-";
                                if (name.substr(0, 1) == '/')
                                    name = name.substr(1, name.length - 1);
                                if (name.includes(prefix) && name.substr(0, prefix.length) == prefix)
                                    existingContainers.push(name.substr(prefix.length, name.length - prefix.length));
                                break;
                            }
                        });
                        const basePath = "/opt/purecore/";
                        const hostedPath = basePath + "hosted/";
                        const tempPath = basePath + "tmp/";
                        if (!fs.existsSync(basePath))
                            fs.mkdirSync(basePath);
                        if (!fs.existsSync(hostedPath))
                            fs.mkdirSync(hostedPath);
                        if (!fs.existsSync(tempPath))
                            fs.mkdirSync(tempPath);
                        fs.readdirSync(hostedPath).filter((dirent) => dirent.isDirectory()).forEach(folder => {
                            if (!existingContainers.includes(folder.name)) {
                                fs.rename(hostedPath + folder.name + "/", tempPath + "noncorrelated-" + folder.name + "/", function (err) {
                                    if (err)
                                        reject(err);
                                });
                            }
                        });
                        resolve();
                    }
                    ;
                });
            }
            catch (err) {
                reject(err);
            }
        });
    }
}
const si = require('systeminformation');
class HardwareCheck {
    static updateComponents() {
        return new Promise(function (resolve, reject) {
            si.getStaticData(function (components) {
                Supervisor.getMachine().updateComponents(components).then(() => {
                    resolve();
                }).catch((err) => {
                    reject(err);
                });
            });
        });
    }
}
const publicIp = require('public-ip');
class NetworkCheck {
    static updateNetwork() {
        return new Promise(function (resolve, reject) {
            publicIp.v4().then(function (ip) {
                Supervisor.getMachine().setIPV4(ip).then(() => {
                    let ipv6Skip = false;
                    setTimeout(() => {
                        if (!ipv6Skip) {
                            ipv6Skip = true;
                            resolve();
                        }
                    }, 2000);
                    publicIp.v6().then(function (ip) {
                        if (!ipv6Skip) {
                            Supervisor.getMachine().setIPV6(ip).then(() => {
                                ipv6Skip = true;
                                resolve();
                            }).catch(() => {
                                // ipv6 is not that relevant, not a big deal
                                ipv6Skip = true;
                                resolve();
                            });
                        }
                    }).catch(() => {
                        // ipv6 is not that relevant, not a big deal
                        if (!ipv6Skip) {
                            ipv6Skip = true;
                            resolve();
                        }
                    });
                });
            }).catch((err) => {
                reject(err);
            });
        });
    }
}
const colors = require('colors');
const inquirer = require('inquirer');
class ConsoleUtil {
    static showTitle() {
        console.log(colors.white("                                                         _                "));
        console.log(colors.white("                                                        (_)               "));
        console.log(colors.white("   ___ ___  _ __ ___     ___ _   _ _ __   ___ _ ____   ___ ___  ___  _ __ "));
        console.log(colors.white("  / __/ _ \\| '__/ _ \\   / __| | | | '_ \\ / _ \\ '__\\ \\ / / / __|/ _ \\| '__|"));
        console.log(colors.white(" | (_| (_) | | |  __/_  \\__ \\ |_| | |_) |  __/ |   \\ V /| \\__ \\ (_) | |   "));
        console.log(colors.white("  \\___\\___/|_|  \\___(_) |___/\\__,_| .__/ \\___|_|    \\_/ |_|___/\\___/|_|   "));
        console.log(colors.white("                                  | |                                     "));
        console.log(colors.white("                                  |_|                                     "));
        console.log("");
        console.log(colors.white("     ◢ by © quiquelhappy "));
        console.log("");
        console.log("");
    }
    static askHash() {
        var questions = [{
                type: 'password',
                name: 'hash',
                prefix: colors.bgMagenta(" ? "),
                message: colors.magenta("Please, enter your machine hash and press enter"),
            }];
        return new Promise(function (resolve, reject) {
            inquirer.prompt(questions).then(answers => {
                resolve(answers['hash']);
            }).catch(() => {
                reject();
            });
        });
    }
    static setLoading(loading, string, failed = false) {
        if (loading) {
            ConsoleUtil.loadingInterval = setInterval(() => {
                ConsoleUtil.loadingStep++;
                process.stdout.clearLine(0);
                process.stdout.cursorTo(0);
                switch (ConsoleUtil.loadingStep) {
                    case 1:
                        ConsoleUtil.character = " ▖ ";
                        break;
                    case 2:
                        ConsoleUtil.character = " ▘ ";
                        break;
                    case 3:
                        ConsoleUtil.character = " ▝ ";
                        break;
                    default:
                        ConsoleUtil.character = " ▗ ";
                        ConsoleUtil.loadingStep = 0;
                        break;
                }
                process.stdout.write(colors.bgMagenta(ConsoleUtil.character) + colors.magenta(" " + string));
            }, 100);
        }
        else {
            clearInterval(ConsoleUtil.loadingInterval);
            process.stdout.clearLine(0);
            process.stdout.cursorTo(0);
            if (!failed) {
                process.stdout.write(colors.bgGreen(" ✓ ") + colors.green(" " + string));
            }
            else {
                process.stdout.write(colors.bgRed(" ☓ ") + colors.red(" " + string));
            }
            process.stdout.write("\n");
            process.stdout.cursorTo(0);
        }
    }
}
ConsoleUtil.loadingInterval = null;
ConsoleUtil.loadingStep = 0;
ConsoleUtil.character = null;
module.exports.ConsoleUtil = ConsoleUtil;
class LiteEvent {
    constructor() {
        this.handlers = [];
    }
    on(handler) {
        this.handlers.push(handler);
    }
    off(handler) {
        this.handlers = this.handlers.filter(h => h !== handler);
    }
    trigger(data) {
        this.handlers.slice(0).forEach(h => h(data));
    }
    expose() {
        return this;
    }
}
class MachineSettings {
    static setHash(hash) {
        return new Promise(function (resolve, reject) {
            try {
                MachineSettings.checkExistance().then(() => {
                    let data = JSON.stringify({ hash: hash });
                    fs.writeFileSync(MachineSettings.basePath + 'settings.json', data);
                    resolve();
                }).catch((error) => {
                    reject(error);
                });
            }
            catch (error) {
                reject(error);
            }
        });
    }
    static getHash() {
        return new Promise(function (resolve, reject) {
            try {
                MachineSettings.checkExistance().then(() => {
                    let rawdata = fs.readFileSync(MachineSettings.basePath + 'settings.json');
                    MachineSettings.hash = JSON.parse(rawdata).hash;
                    resolve(MachineSettings.hash);
                }).catch((error) => {
                    reject(error);
                });
            }
            catch (error) {
                reject(error);
            }
        });
    }
    static checkExistance() {
        return new Promise(function (resolve, reject) {
            try {
                if (!fs.existsSync(MachineSettings.basePath))
                    fs.mkdirSync(MachineSettings.basePath);
                if (!fs.existsSync(MachineSettings.basePath + "settings.json")) {
                    fs.writeFile(MachineSettings.basePath + 'settings.json', "{ \"hash\":null}", function (err) {
                        if (err)
                            reject(err);
                        MachineSettings.hash = null;
                        resolve();
                    });
                }
                else {
                    resolve();
                }
            }
            catch (error) {
                reject(error);
            }
        });
    }
}
MachineSettings.hash = null;
MachineSettings.basePath = "/opt/purecore/";
