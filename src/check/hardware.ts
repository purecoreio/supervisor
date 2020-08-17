const si = require('systeminformation');

class HardwareCheck {

    static updateComponents(): Promise<void> {
        return new Promise(function (resolve, reject) {
            si.getStaticData(function (components) {
                Supervisor.getMachine().updateComponents(components).then(() => {
                    resolve()
                }).catch((err) => {
                    reject(err);
                })
            })
        });
    }

}