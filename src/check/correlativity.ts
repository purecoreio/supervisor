const dockerode = require('dockerode');
const fs = require('fs');

class Correlativity {

    /**
     * Will move the folders on /opt/purecore/hosted/ to /opt/purecore/tmp/ when no purecore.io docker is matching that data
     */
    static updateFolders(): Promise<void> {
        return new Promise(function (resolve, reject) {
            try {
                var docker = new dockerode();
                docker.listContainers(function (err, containers) {
                    let existingContainers = [];
                    
                    if (containers == null) { reject() } else {

                        containers.forEach(function (containerInfo) {
                            for (var name of docker.getContainer(containerInfo.Id).Names) {
                                name = String(name);
                                const prefix = "core-";
                                if (name.substr(0, 1) == '/') name = name.substr(1, name.length - 1)
                                if (name.includes(prefix) && name.substr(0, prefix.length) == prefix) existingContainers.push(name.substr(prefix.length, name.length - prefix.length)); break;
                            }
                        });

                        const basePath = "/opt/purecore/";
                        const hostedPath = basePath + "hosted/";
                        const tempPath = basePath + "tmp/";

                        if (!fs.existsSync(basePath)) fs.mkdirSync(basePath)
                        if (!fs.existsSync(hostedPath)) fs.mkdirSync(hostedPath)
                        if (!fs.existsSync(tempPath)) fs.mkdirSync(tempPath)

                        fs.readdirSync(hostedPath).filter((dirent) => dirent.isDirectory()).forEach(folder => {
                            if (!existingContainers.includes(folder.name)) {
                                fs.rename(hostedPath + folder.name + "/", tempPath + "noncorrelated-" + folder.name + "/", function (err) {
                                    if (err) reject(err);
                                })
                            }
                        });
                        resolve();
                    };

                })
            } catch (err) {
                reject(err);
            }
        });
    }

}