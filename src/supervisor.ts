const core = require('purecore');
const EventEmitter = require('events');

class Supervisor {

    // events
    public static emitter = new EventEmitter();

    // actual props
    public static hash: string = null;
    public static ready: boolean = false;
    public static machine;

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

        if (hash != null) {
            await MachineSettings.setHash(hash).catch(() => {
                Supervisor.emitter.emit('hashSavingError');
            })
            Supervisor.hash = hash;
        } else {
            await MachineSettings.getHash().then((storedHash) => {
                Supervisor.hash = storedHash;
            }).catch(() => {
                Supervisor.emitter.emit('hashLoadingError');
            })
        }

        Supervisor.emitter.emit('loadingMachine');
        new core().getMachine(Supervisor.hash).then((machine) => {
            Supervisor.emitter.emit('gotMachine')
            Supervisor.machine = machine;
            main.IOCheck().then(() => {
                Supervisor.emitter.emit('checkingCorrelativity');
                Correlativity.updateFolders().then(() => {
                    Supervisor.emitter.emit('checkedCorrelativity');
                    try {
                        new SocketServer().setup();
                    } catch (error) {
                        Supervisor.emitter.emit('errorSettingUpSockets');
                    }
                }).catch(() => {
                    Supervisor.emitter.emit('errorCheckingCorrelativity');
                })
            }).catch((err) => {
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