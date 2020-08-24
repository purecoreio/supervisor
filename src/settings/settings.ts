class MachineSettings {

    public static hash: string = null;
    public static basePath = "/etc/purecore/";

    public static setHash(hash): Promise<void> {
        return new Promise(function (resolve, reject) {
            try {
                MachineSettings.checkExistance().then(() => {
                    let data = JSON.stringify({ hash: hash });
                    fs.writeFileSync(MachineSettings.basePath + 'settings.json', data);
                    resolve();
                }).catch((error) => {
                    reject(error);
                })
            } catch (error) {
                reject(error);
            }
        });
    }

    public static getHash(): Promise<string> {
        return new Promise(function (resolve, reject) {
            try {
                MachineSettings.checkExistance().then(() => {
                    let rawdata = fs.readFileSync(MachineSettings.basePath + 'settings.json');
                    MachineSettings.hash = JSON.parse(rawdata).hash;
                    resolve(MachineSettings.hash);
                }).catch((error) => {
                    reject(error);
                })
            } catch (error) {
                reject(error);
            }
        });
    }

    public static checkExistance(): Promise<void> {
        return new Promise(function (resolve, reject) {
            try {
                if (!fs.existsSync("/etc/")) fs.mkdirSync("/etc/")
                if (!fs.existsSync(MachineSettings.basePath)) fs.mkdirSync(MachineSettings.basePath)
                if (!fs.existsSync(MachineSettings.basePath + "settings.json")) {
                    fs.writeFile(MachineSettings.basePath + 'settings.json', "{ \"hash\":null}", function (err) {
                        if (err) reject(err);
                        MachineSettings.hash = null;
                        resolve();
                    })
                } else {
                    resolve();
                }
            } catch (error) {
                reject(error);
            }
        });
    }

}