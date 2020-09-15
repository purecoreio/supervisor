const core = require('purecore');
const EventEmitter = require('events');
const dockerode = require('dockerode');

class Supervisor {

    // events
    public static emitter = new EventEmitter();
    public static docker = null;

    // actual props
    public static hash: string = null;
    public static ready: boolean = false;
    public static machine;

    public static hostAuths: Array<any> = [];

    constructor(hash: string) {
        if (Supervisor.hash != hash && hash != null) {
            MachineSettings.getHash().then((hash) => {
                Supervisor.hash = hash;
            });
        }
    }

    public getEmitter(): any {
        return Supervisor.emitter;
    }

    public async setup(hash?: string) {
        let main = this;

        try {
            if (Supervisor.docker == null) {
                Supervisor.emitter.emit('dockerComStart')
                Supervisor.docker = new dockerode();
            }
        } catch (error) {
            Supervisor.emitter.emit('dockerComError')
        }


        if (hash != null) {
            await MachineSettings.setHash(hash).catch(() => {
                Supervisor.emitter.emit('hashSavingError');
            })
            Supervisor.hash = hash;
        } else {
            await MachineSettings.getHash().then((storedHash) => {
                Supervisor.hash = storedHash;
            }).catch((err) => {
                Supervisor.emitter.emit('hashLoadingError', err);
            })
        }

        Supervisor.emitter.emit('loadingMachine');
        new core().getMachine(Supervisor.hash).then((machine) => {
            Supervisor.emitter.emit('gotMachine')
            Supervisor.machine = machine;
            main.IOCheck().then(() => {

                Supervisor.emitter.emit('gettingHosts');
                Supervisor.machine.getHostAuths().then((hosts) => {
                    Supervisor.emitter.emit('gotHosts');
                    Supervisor.hostAuths = hosts;
                    Supervisor.emitter.emit('checkingCorrelativity');
                    Correlativity.updateContainers().then(() => {
                        Supervisor.emitter.emit('checkedCorrelativity');
                        sshdCheck.applyConfig().then(() => {
                            DockerLogger.pushAllExistingContainers().then(() => {
                                try {
                                    new SocketClient().setup();
                                } catch (error) {
                                    Supervisor.emitter.emit('errorSettingUpSockets');
                                }
                            }).catch((err) => {
                                // ignore
                            })
                        })
                    }).catch((err) => {
                        Supervisor.emitter.emit('errorCheckingCorrelativity', err);
                    })
                }).catch(() => {
                    Supervisor.emitter.emit('errorGettingHosts');
                })
            }).catch(() => {
                // can't complete the setup process
            })
        }).catch((err) => {
            Supervisor.emitter.emit('errorGettingMachine', err)
        })
    }

    public static getSupervisor() {
        return Supervisor;
    }

    public static getMachine() {
        return Supervisor.machine;
    }

    public IOCheck(): Promise<void> {

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
                })
            }).catch((err) => {
                Supervisor.emitter.emit('errorPushingHardware');
                reject(err);
            })
        });
    }

}

module.exports.Supervisor = Supervisor;