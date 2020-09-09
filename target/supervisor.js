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
const dockerode = require('dockerode');
let Supervisor = /** @class */ (() => {
    class Supervisor {
        constructor(hash) {
            if (Supervisor.hash != hash && hash != null) {
                MachineSettings.getHash().then((hash) => {
                    Supervisor.hash = hash;
                });
            }
        }
        getEmitter() {
            return Supervisor.emitter;
        }
        setup(hash) {
            return __awaiter(this, void 0, void 0, function* () {
                let main = this;
                try {
                    if (Supervisor.docker == null) {
                        Supervisor.emitter.emit('dockerComStart');
                        Supervisor.docker = new dockerode();
                    }
                }
                catch (error) {
                    Supervisor.emitter.emit('dockerComError');
                }
                if (hash != null) {
                    yield MachineSettings.setHash(hash).catch(() => {
                        Supervisor.emitter.emit('hashSavingError');
                    });
                    Supervisor.hash = hash;
                }
                else {
                    yield MachineSettings.getHash().then((storedHash) => {
                        Supervisor.hash = storedHash;
                    }).catch((err) => {
                        Supervisor.emitter.emit('hashLoadingError', err);
                    });
                }
                Supervisor.emitter.emit('loadingMachine');
                new core().getMachine(Supervisor.hash).then((machine) => {
                    Supervisor.emitter.emit('gotMachine');
                    Supervisor.machine = machine;
                    main.IOCheck().then(() => {
                        Supervisor.emitter.emit('gettingHosts');
                        Supervisor.machine.getHostAuths().then((hosts) => {
                            Supervisor.emitter.emit('gotHosts');
                            Supervisor.hostAuths = hosts;
                            Supervisor.emitter.emit('checkingCorrelativity');
                            Correlativity.updateFolders().then(() => {
                                Supervisor.emitter.emit('checkedCorrelativity');
                                sshdCheck.applyConfig().then(() => {
                                    try {
                                        new SocketServer().setup();
                                    }
                                    catch (error) {
                                        Supervisor.emitter.emit('errorSettingUpSockets');
                                    }
                                });
                            }).catch((err) => {
                                Supervisor.emitter.emit('errorCheckingCorrelativity', err);
                            });
                        }).catch(() => {
                            Supervisor.emitter.emit('errorGettingHosts');
                        });
                    }).catch((err) => {
                        // can't complete the setup process
                    });
                }).catch((err) => {
                    Supervisor.emitter.emit('errorGettingMachine', err);
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
                Supervisor.emitter.emit('pushingHardware');
                HardwareCheck.updateComponents().then(() => {
                    Supervisor.emitter.emit('pushedHardware');
                    Supervisor.emitter.emit('pushingNetwork');
                    NetworkCheck.updateNetwork().then(() => {
                        Supervisor.emitter.emit('pushedNetwork');
                        resolve();
                    }).catch((err) => {
                        Supervisor.emitter.emit('errorPushingNetwork');
                        reject(err);
                    });
                }).catch((err) => {
                    Supervisor.emitter.emit('errorPushingHardware');
                    reject(err);
                });
            });
        }
    }
    // events
    Supervisor.emitter = new EventEmitter();
    Supervisor.docker = null;
    // actual props
    Supervisor.hash = null;
    Supervisor.ready = false;
    Supervisor.hostAuths = [];
    return Supervisor;
})();
module.exports.Supervisor = Supervisor;
const fs = require('fs');
const cryptotool = require("crypto");
let Correlativity = /** @class */ (() => {
    class Correlativity {
        static checkFilesystem(existingContainers) {
            return new Promise(function (resolve, reject) {
                try {
                    let folders = [];
                    let actionsToTake = 0;
                    fs.readdirSync(Correlativity.hostedPath).forEach(folder => {
                        if (fs.lstatSync(Correlativity.hostedPath + folder).isDirectory()) {
                            folders.push(folder);
                            let found = false;
                            existingContainers.forEach(existingContainer => {
                                if (existingContainer.name == folder) {
                                    found = true;
                                }
                            });
                            if (!found) {
                                actionsToTake++;
                                sshdCheck.removeUser(folder).catch(() => {
                                    //ignore
                                });
                                fs.rename(Correlativity.hostedPath + folder + "/", Correlativity.tempPath + "noncorrelated-" + cryptotool.randomBytes(8).toString('hex') + "-" + folder + "/", function (err) {
                                    actionsToTake += -1;
                                    if (err) {
                                        Supervisor.emitter.emit('errorMovingUncorrelatedFolder', new Error(err.code));
                                        reject(err);
                                    }
                                    else {
                                        Supervisor.emitter.emit('movedUncorrelatedFolder');
                                    }
                                    if (actionsToTake <= 0) {
                                        resolve();
                                    }
                                });
                            }
                        }
                    });
                    if (actionsToTake <= 0) {
                        resolve();
                    }
                }
                catch (err) {
                    reject(err);
                }
            });
        }
        static updateFolders() {
            return new Promise(function (resolve, reject) {
                try {
                    Supervisor.docker.listContainers({ all: true }, function (err, containers) {
                        let existingContainers = [];
                        if (containers == null) {
                            reject(new Error("couldn't communicate with Docker"));
                        }
                        else {
                            containers.forEach(function (containerInfo) {
                                for (var name of containerInfo.Names) {
                                    name = String(name);
                                    const prefix = "core-";
                                    if (name.substr(0, 1) == '/')
                                        name = name.substr(1, name.length - 1);
                                    if (name.includes(prefix) && name.substr(0, prefix.length) == prefix)
                                        existingContainers.push({ name: name.substr(prefix.length, name.length - prefix.length), id: containerInfo.Id });
                                    break;
                                }
                            });
                            if (!fs.existsSync(Correlativity.basePath))
                                fs.mkdirSync(Correlativity.basePath);
                            if (!fs.existsSync(Correlativity.hostedPath))
                                fs.mkdirSync(Correlativity.hostedPath);
                            if (!fs.existsSync(Correlativity.tempPath))
                                fs.mkdirSync(Correlativity.tempPath);
                            let actionsToTake = 0;
                            let existingContainerIds = [];
                            Supervisor.hostAuths.forEach(auth => {
                                existingContainerIds.push(auth.host.uuid);
                            });
                            if (existingContainers.length <= 0) {
                                Correlativity.checkFilesystem(existingContainers).then(() => {
                                    resolve();
                                }).catch((err) => {
                                    reject(err);
                                });
                            }
                            else {
                                existingContainers.forEach(containerInfo => {
                                    if (!existingContainerIds.includes(containerInfo.name)) {
                                        // remove from existing containers (about to be deleted)
                                        sshdCheck.removeUser(containerInfo.name).catch(() => {
                                            //ignore
                                        });
                                        existingContainers = existingContainers.filter(function (returnableObjects) {
                                            return returnableObjects.name !== containerInfo.name;
                                        });
                                        actionsToTake++;
                                        Supervisor.docker.getContainer(containerInfo.id).remove({
                                            force: true
                                        }, (err) => {
                                            actionsToTake += -1;
                                            if (err) {
                                                Supervisor.emitter.emit('errorRemovingUncorrelatedContainer');
                                                reject(err);
                                            }
                                            else {
                                                Supervisor.emitter.emit('removedUncorrelatedContainer');
                                            }
                                            if (actionsToTake <= 0) {
                                                Correlativity.checkFilesystem(existingContainers).then(() => {
                                                    resolve();
                                                }).catch((err) => {
                                                    reject(err);
                                                });
                                            }
                                        });
                                    }
                                    if (actionsToTake == 0) {
                                        Correlativity.checkFilesystem(existingContainers).then(() => {
                                            resolve();
                                        }).catch((err) => {
                                            reject(err);
                                        });
                                    }
                                });
                            }
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
    /**
     * Will move the folders on /etc/purecore/hosted/ to /etc/purecore/tmp/ when no purecore.io docker is matching that data
     */
    Correlativity.basePath = "/etc/purecore/";
    Correlativity.hostedPath = "/etc/purecore/hosted/";
    Correlativity.tempPath = "/etc/purecore/tmp/";
    return Correlativity;
})();
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
const SSHConfig = require('ssh-config');
const linuxUser = require('linux-sys-user');
const chown = require('chown');
let sshdCheck = /** @class */ (() => {
    class sshdCheck {
        static getCurrentConfig() {
            try {
                let rawdata = fs.readFileSync(sshdCheck.sshdConfigPath, 'utf8');
                const config = SSHConfig.parse(rawdata);
                return config;
            }
            catch (error) {
                Supervisor.emitter.emit('sshdParseError', error);
            }
        }
        static getNewConfig() {
            let config = sshdCheck.getCurrentConfig();
            let addSubsystem = false;
            let chrootRuleFound = false;
            for (let index = 0; index < config.length; index++) {
                let element = config[index];
                if (element.param == 'Subsystem' && element.type != 2) {
                    if (!element.value.includes('sftp') || !element.value.includes('internal-sftp')) {
                        element.type = 2;
                        element.content = `#${element.param}${element.value} [before purecore installation]`;
                        delete element.param;
                        delete element.value;
                        delete element.separator;
                        config[index] = element;
                        addSubsystem = true;
                    }
                }
                if (element.param == 'Match' && element.type != 2) {
                    if (element.value == 'Group purecore')
                        chrootRuleFound = true;
                }
            }
            if (addSubsystem) {
                Supervisor.emitter.emit('sshdPendingSubsystem');
                config.push({
                    type: 1,
                    param: 'Subsystem',
                    separator: '\t',
                    value: 'sftp\tinternal-sftp',
                    before: '',
                    after: '\n\n'
                });
            }
            if (!chrootRuleFound) {
                Supervisor.emitter.emit('sshdPendingChroot');
                config.append({
                    Match: 'Group purecore',
                    ChrootDirectory: '/etc/purecore/hosted/%u',
                    ForceCommand: 'internal-sftp'
                });
            }
            return SSHConfig.stringify(config);
        }
        static createGroupIfNeeded() {
            return __awaiter(this, void 0, void 0, function* () {
                return new Promise(function (resolve, reject) {
                    linuxUser.getGroupInfo('purecore', function (err, data) {
                        if (err || data == null) {
                            Supervisor.emitter.emit('creatingGroup');
                            linuxUser.addGroup('purecore', function (err, data) {
                                if (err) {
                                    Supervisor.emitter.emit('errorCreatingGroup');
                                    reject();
                                }
                                else {
                                    Supervisor.emitter.emit('createdGroup');
                                    resolve();
                                }
                            });
                        }
                        else {
                            resolve();
                        }
                    });
                });
            });
        }
        static createUser(hostAuth) {
            return __awaiter(this, void 0, void 0, function* () {
                console.log("creating user");
                return new Promise(function (resolve, reject) {
                    sshdCheck.createGroupIfNeeded().then(() => {
                        console.log("group present");
                        Supervisor.emitter.emit('creatingUser');
                        const userPath = Correlativity.hostedPath + hostAuth.host.uuid;
                        const dataPath = userPath + "/data";
                        if (!fs.existsSync(userPath))
                            fs.mkdirSync(userPath);
                        if (!fs.existsSync(dataPath))
                            fs.mkdirSync(dataPath);
                        linuxUser.addUser({ username: hostAuth.host.uuid, create_home: true, home_dir: dataPath, shell: null }, function (err, user) {
                            if (err) {
                                console.log("error creating user " + err.message);
                                Supervisor.emitter.emit('errorCreatingUser');
                                reject();
                            }
                            chown(dataPath, hostAuth.host.uuid, 'purecore')
                                .then(() => {
                                console.log("chowned");
                                Supervisor.emitter.emit('createdUser');
                                Supervisor.emitter.emit('addingUserToGroup');
                                linuxUser.addUserToGroup(hostAuth.host.uuid, 'purecore', function (err, user) {
                                    if (err) {
                                        console.log("error adding to group");
                                        Supervisor.emitter.emit('errorAddingUserToGroup');
                                        reject();
                                    }
                                    console.log("added to group");
                                    Supervisor.emitter.emit('addedUserToGroup');
                                    Supervisor.emitter.emit('settingUserPassword');
                                    linuxUser.setPassword(hostAuth.host.uuid, hostAuth.hash, function (err, user) {
                                        if (err) {
                                            console.log("error setting password");
                                            Supervisor.emitter.emit('errorSettingUserPassword');
                                            reject();
                                        }
                                        console.log("set password");
                                        Supervisor.emitter.emit('setUserPassword');
                                        resolve();
                                    });
                                });
                            })
                                .catch((err) => {
                                console.log("error chowning");
                                Supervisor.emitter.emit('errorChowningUser', err);
                            });
                        });
                    }).catch((err) => {
                        console.log("error creating group " + err.message);
                        reject();
                    });
                });
            });
        }
        static removeUser(username) {
            return __awaiter(this, void 0, void 0, function* () {
                return new Promise(function (resolve, reject) {
                    Supervisor.emitter.emit('removingUser');
                    if (typeof username == 'string' && username.length == 16) {
                        linuxUser.removeUser(username, function (err, data) {
                            if (err || data == null) {
                                Supervisor.emitter.emit('errorRemovingUser');
                                reject();
                            }
                            else {
                                Supervisor.emitter.emit('removedUser');
                                resolve();
                            }
                        });
                    }
                    else {
                        reject();
                    }
                });
            });
        }
        static applyConfig() {
            return new Promise(function (resolve, reject) {
                sshdCheck.createGroupIfNeeded().then(() => {
                    let newConfig = sshdCheck.getNewConfig();
                    if (SSHConfig.stringify(sshdCheck.getCurrentConfig()) != newConfig) {
                        Supervisor.emitter.emit('sshdConfigurationChanging');
                        fs.writeFile(sshdCheck.sshdConfigPath, newConfig, 'utf8', function (err) {
                            if (err) {
                                Supervisor.emitter.emit('sshdConfigurationChangeError');
                                reject();
                            }
                            Supervisor.emitter.emit('sshdConfigurationChanged');
                            resolve();
                        });
                    }
                    else {
                        resolve();
                    }
                }).catch(() => {
                    Supervisor.emitter.emit('errorCreatingGroup');
                });
            });
        }
    }
    sshdCheck.sshdConfigPath = "/etc/ssh/sshd_config";
    return sshdCheck;
})();
const colors = require('colors');
const inquirer = require('inquirer');
let ConsoleUtil = /** @class */ (() => {
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
        static setLoading(loading, string, failed = false, warning = false, info = false) {
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
                if (!info) {
                    if (!warning) {
                        if (!failed) {
                            process.stdout.write(colors.bgGreen(" ✓ ") + colors.green(" " + string));
                        }
                        else {
                            process.stdout.write(colors.bgRed(" ☓ ") + colors.red(" " + string));
                        }
                    }
                    else {
                        process.stdout.write(colors.bgYellow(colors.black(" ⚠ ")) + colors.yellow(" " + string));
                    }
                }
                else {
                    process.stdout.write(colors.bgBlue(" ℹ ") + colors.blue(" " + string));
                }
                process.stdout.write("\n");
                process.stdout.cursorTo(0);
            }
        }
    }
    ConsoleUtil.loadingInterval = null;
    ConsoleUtil.loadingStep = 0;
    ConsoleUtil.character = null;
    return ConsoleUtil;
})();
module.exports.ConsoleUtil = ConsoleUtil;
const { PassThrough } = require('stream');
let DockerHelper = /** @class */ (() => {
    class DockerHelper {
        static getContainer(host) {
            return __awaiter(this, void 0, void 0, function* () {
                return new Promise(function (resolve, reject) {
                    Supervisor.docker.listContainers(function (err, containers) {
                        for (let index = 0; index < containers.length; index++) {
                            const element = containers[index];
                            for (var name of element.Names) {
                                name = String(name);
                                const prefix = "core-";
                                if (name.substr(0, 1) == '/')
                                    name = name.substr(1, name.length - 1);
                                if (name.includes(prefix) && name.substr(0, prefix.length) == prefix && name.substr(prefix.length, name.length - prefix.length) == host.uuid) {
                                    resolve(Supervisor.docker.getContainer(element.Id));
                                    break;
                                }
                            }
                        }
                    });
                });
            });
        }
        static getLogStream(container) {
            return __awaiter(this, void 0, void 0, function* () {
                return new Promise(function (resolve, reject) {
                    const logStream = new PassThrough();
                    if (container != null) {
                        container.logs({
                            follow: true,
                            stdout: true,
                            stderr: true
                        }, function (err, stream) {
                            container.modem.demuxStream(stream, logStream, logStream);
                            stream.on('end', function () {
                                logStream.end('!stop');
                            });
                            resolve(logStream);
                        });
                    }
                    else {
                        throw new Error("Unknown container");
                    }
                });
            });
        }
        static actuallyCreateContainer(retrypquota, opts) {
            let promise = new Promise(function (resolve, reject) {
                Supervisor.docker.createContainer(opts).then((container) => {
                    Supervisor.emitter.emit('createdContainer');
                    Supervisor.emitter.emit('startingNewContainer');
                    container.start().then(() => {
                        Supervisor.emitter.emit('startedNewContainer');
                        resolve(null);
                    });
                }).catch((error) => {
                    if (retrypquota && error.message.includes('pquota')) {
                        ConsoleUtil.setLoading(false, "Creating container with no size limit, please, use the overlay2 storage driver, back it with extfs, enable d_type and make sure pquota is available (probably your issue, read more here: https://stackoverflow.com/a/57248363/7280257)", false, true, false);
                        delete opts.HostConfig.StorageOpt;
                        let newPromise = DockerHelper.actuallyCreateContainer(false, opts);
                        resolve(newPromise);
                    }
                    else {
                        reject(error);
                    }
                });
            });
            return promise;
        }
        static createContainer(authRequest) {
            authRequest = Supervisor.machine.core.getHostingManager().getHostAuth().fromObject(authRequest);
            if (!Supervisor.hostAuths.includes(authRequest)) {
                Supervisor.hostAuths.push(authRequest);
            }
            return new Promise(function (resolve, reject) {
                Supervisor.emitter.emit('creatingContainer');
                const basePath = "/etc/purecore/";
                const hostedPath = basePath + "hosted/";
                let opts = {
                    Image: authRequest.host.image, name: 'core-' + authRequest.host.uuid,
                    Env: [
                        "EULA=true",
                        "MEMORY=" + authRequest.host.template.memory
                    ],
                    HostConfig: {
                        PortBindings: {
                            '25565/tcp': [
                                { HostPort: String(authRequest.host.port) }
                            ]
                        },
                        Memory: authRequest.host.template.memory,
                        RestartPolicy: {
                            name: 'unless-stopped',
                        },
                        StorageOpt: {
                            size: `${authRequest.host.template.size / 1073741824}G`
                        },
                        Binds: [
                            `${hostedPath}/${authRequest.host.uuid}/data:/data`
                        ],
                        NanoCpus: authRequest.host.template.cores * 10 ^ 9
                    },
                };
                try {
                    DockerHelper.actuallyCreateContainer(true, opts).then((res) => {
                        if (res == null) {
                            Supervisor.emitter.emit('registeringUser');
                            sshdCheck.createUser(authRequest).then(() => {
                                Supervisor.emitter.emit('registeredUser');
                                resolve();
                            }).catch(() => {
                                Supervisor.emitter.emit('errorUserRegistration');
                            });
                        }
                        else {
                            res.then(() => {
                                Supervisor.emitter.emit('registeringUser');
                                sshdCheck.createUser(authRequest).then(() => {
                                    Supervisor.emitter.emit('registeredUser');
                                    resolve();
                                }).catch(() => {
                                    Supervisor.emitter.emit('errorUserRegistration');
                                });
                            }).catch((err) => {
                                Supervisor.emitter.emit('containerCreationError', err);
                                reject();
                            });
                        }
                    }).catch((err) => {
                        Supervisor.emitter.emit('containerCreationError', err);
                        reject();
                    });
                }
                catch (error) {
                    Supervisor.emitter.emit('containerCreationError', error);
                    reject();
                }
            });
        }
    }
    DockerHelper.hostingFolder = "/etc/purecore/hosted/";
    return DockerHelper;
})();
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
class Cert {
    constructor(key, cert, ca) {
        this.key = key;
        this.key = cert;
        this.ca = ca;
    }
}
const http = require('http');
const https = require('https');
const socketio = require('socket.io');
const app = require('express')();
let SocketServer = /** @class */ (() => {
    class SocketServer {
        getSocket(server) {
            return new socketio(server).on('connection', client => {
                client.on('authenticate', authInfo => {
                    this.authenticate(client, authInfo);
                });
                client.on('console', extra => {
                    if (SocketServer.getHost(client) != null && SocketServer.isAuthenticated(client)) {
                        try {
                            DockerHelper.getContainer(SocketServer.getHost(client).host).then((container) => {
                                try {
                                    DockerHelper.getLogStream(container).then((logStream) => {
                                        logStream.on('data', (data) => {
                                            if (!client.connected) {
                                                logStream.end();
                                            }
                                            else {
                                                try {
                                                    client.emit('console', data.toString('utf-8').trim());
                                                }
                                                catch (error) {
                                                    logStream.end();
                                                }
                                            }
                                        }).on('error', () => {
                                            logStream.end();
                                        });
                                    });
                                }
                                catch (error) {
                                    // ignore
                                }
                            });
                        }
                        catch (error) {
                            console.log(error);
                        }
                    }
                });
                client.on('host', hostObject => {
                    if (SocketServer.isMasterAuthenticated(client))
                        DockerHelper.createContainer(hostObject).catch((err) => { });
                });
                client.on('disconnect', () => { SocketServer.removeAuth(client.id); });
            });
        }
        static getHost(client) {
            for (let index = 0; index < SocketServer.authenticatedHosts.length; index++) {
                const element = SocketServer.authenticatedHosts[index];
                if (element.client == client.id)
                    return element.hostAuth;
                break;
            }
        }
        static isMasterAuthenticated(client) {
            return SocketServer.authenticated.includes(client.id);
        }
        static isAuthenticated(client) {
            if (SocketServer.authenticated.includes(client.id)) {
                return true;
            }
            else {
                for (let index = 0; index < SocketServer.authenticatedHosts.length; index++) {
                    const element = SocketServer.authenticatedHosts[index];
                    if (element.client == client.id)
                        return true;
                    break;
                }
            }
        }
        static addAuth(client, host) {
            if (host == null) {
                if (!SocketServer.authenticated.includes(client.id)) {
                    Supervisor.emitter.emit('clientConnected');
                    this.authenticated.push(client.id);
                }
            }
            else {
                if (!SocketServer.authenticatedHosts.includes(client.id)) {
                    Supervisor.emitter.emit('clientConnected');
                    SocketServer.authenticatedHosts.push({ client: client.id, hostAuth: host });
                }
            }
            client.emit('authenticated');
        }
        static removeAuth(clientid) {
            if (this.isAuthenticated(clientid)) {
                Supervisor.emitter.emit('clientDisconnected');
                SocketServer.authenticated = SocketServer.authenticated.filter(x => x !== clientid);
                SocketServer.authenticatedHosts = SocketServer.authenticatedHosts.filter(x => x.client !== clientid);
            }
        }
        authenticate(client, authInfo) {
            if ((authInfo == null || authInfo == "" || authInfo == [] || authInfo == {})) {
                const accepetedHostnames = ["api.purecore.io", "purecore.io"];
                const hostname = client.handshake.headers.host.split(".").shift();
                if (accepetedHostnames.includes(hostname)) {
                    SocketServer.addAuth(client);
                }
                else {
                    client.disconnect();
                }
            }
            else if (typeof authInfo == "object") {
                if ('hash' in authInfo && authInfo.hash == Supervisor.machine.hash) {
                    SocketServer.addAuth(client);
                }
                else if ('auth' in authInfo) {
                    try {
                        let authHash = authInfo.auth;
                        let match = null;
                        for (let index = 0; index < Supervisor.hostAuths.length; index++) {
                            const element = Supervisor.hostAuths[index];
                            if (element.hash == authHash) {
                                match = element;
                                break;
                            }
                        }
                        if (match != null) {
                            SocketServer.addAuth(client, match);
                        }
                        else {
                            client.disconnect();
                        }
                    }
                    catch (error) {
                        client.disconnect();
                    }
                }
                else {
                    client.disconnect();
                }
            }
            else {
                client.disconnect();
            }
        }
        setup() {
            try {
                Supervisor.emitter.emit('creatingServer');
                const server = this.getHTTP();
                Supervisor.emitter.emit('createdServer');
                try {
                    Supervisor.emitter.emit('creatingSocketServer');
                    SocketServer.io = this.getSocket(server);
                    server.listen(31518, () => {
                        Supervisor.emitter.emit('createdSocketServer');
                    }).on('error', function (error) {
                        Supervisor.emitter.emit('errorCreatingSocketServer', new Error(error.code));
                    });
                }
                catch (error) {
                    Supervisor.emitter.emit('errorCreatingSocketServer', error);
                }
            }
            catch (error) {
                Supervisor.emitter.emit('errorCreatingServer', error);
            }
        }
        getHTTP() {
            let httpServer = null;
            const cert = this.getCert();
            if (cert != null) {
                Supervisor.emitter.emit('certUse');
                httpServer = https.Server({
                    key: cert.key,
                    cert: cert.cert,
                    ca: cert.ca
                }, app);
            }
            else {
                Supervisor.emitter.emit('certUnknown');
            }
            if (httpServer == null) {
                httpServer = http.createServer();
            }
            return httpServer;
        }
        getCert() {
            Supervisor.emitter.emit('certLoading');
            const fs = require("fs");
            const letsencryptBase = "/etc/letsencrypt/";
            try {
                if (fs.existsSync(letsencryptBase) && fs.existsSync(letsencryptBase + "live/")) {
                    let files = fs.readdirSync(letsencryptBase + "live/");
                    let folders = [];
                    files.forEach(file => {
                        if (fs.lstatSync(letsencryptBase + file).isDirectory())
                            folders.push(file);
                    });
                    if (folders != null && Array.isArray(folders) && folders.length > 0) {
                        Supervisor.emitter.emit('certFound');
                        return new Cert(fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/privkey.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/cert.pem"), fs.readFileSync(letsencryptBase + "live/" + folders[0] + "/chain.pem"));
                    }
                    else {
                        Supervisor.emitter.emit('certNotSetup');
                        return null;
                    }
                }
                else {
                    Supervisor.emitter.emit('certNotInstalled');
                    return null;
                }
            }
            catch (error) {
                Supervisor.emitter.emit('certReadingError');
                return null;
            }
        }
    }
    SocketServer.authenticated = [];
    SocketServer.authenticatedHosts = [];
    return SocketServer;
})();
let MachineSettings = /** @class */ (() => {
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
                    if (!fs.existsSync("/etc/"))
                        fs.mkdirSync("/etc/");
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
    MachineSettings.basePath = "/etc/purecore/";
    return MachineSettings;
})();
