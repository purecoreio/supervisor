const core = require('purecore');
const EventEmitter = require('events');

class Supervisor {

    // events
    public emitter = new EventEmitter();

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

    public async setup(hash?: string) {
        let main = this;

        if (hash != null) {
            await MachineSettings.setHash(hash).catch(() => {
                main.emitter.emit('hashSavingError');
            })
            Supervisor.hash = hash;
        } else {
            await MachineSettings.getHash().then((storedHash) => {
                Supervisor.hash = storedHash;
            }).catch(() => {
                main.emitter.emit('hashLoadingError');
            })
        }

        main.emitter.emit('loadingMachine');
        new core().getMachine(Supervisor.hash).then((machine) => {
            main.emitter.emit('gotMachine')
            Supervisor.machine = machine;
            main.IOCheck().then(() => {
                main.emitter.emit('checkingCorrelativity');
                Correlativity.updateFolders().then(() => {
                    main.emitter.emit('checkedCorrelativity');
                }).catch(() => {
                    main.emitter.emit('errorCheckingCorrelativity');
                })
            }).catch((err) => {
                // can't complete the setup process
            })
        }).catch((err) => {
            main.emitter.emit('errorGettingMachine', err)
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
                })
            }).catch((err) => {
                main.emitter.emit('errorPushingHardware');
                reject(err);
            })
        });
    }

}

module.exports.Supervisor = Supervisor;