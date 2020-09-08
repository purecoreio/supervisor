const fs = require('fs');
const cryptotool = require("crypto");

class Correlativity {

    /**
     * Will move the folders on /etc/purecore/hosted/ to /etc/purecore/tmp/ when no purecore.io docker is matching that data
     */
    static updateFolders(): Promise<void> {
        return new Promise(function (resolve, reject) {
            try {
                Supervisor.docker.listContainers({ all: true }, function (err, containers) {
                    let existingContainers = [];

                    if (containers == null) { reject(new Error("couldn't communicate with Docker")) } else {

                        containers.forEach(function (containerInfo) {
                            for (var name of containerInfo.Names) {
                                name = String(name);
                                const prefix = "core-";
                                if (name.substr(0, 1) == '/') name = name.substr(1, name.length - 1);
                                if (name.includes(prefix) && name.substr(0, prefix.length) == prefix) existingContainers.push({ name: name.substr(prefix.length, name.length - prefix.length), id: containerInfo.Id }); break;
                            }
                        });

                        const basePath = "/etc/purecore/";
                        const hostedPath = basePath + "hosted/";
                        const tempPath = basePath + "tmp/";


                        if (!fs.existsSync(basePath)) fs.mkdirSync(basePath)
                        if (!fs.existsSync(hostedPath)) fs.mkdirSync(hostedPath)
                        if (!fs.existsSync(tempPath)) fs.mkdirSync(tempPath)

                        let folders = [];
                        let actionsToTake = 0;

                        fs.readdirSync(hostedPath).forEach(folder => {
                            if (fs.lstatSync(hostedPath + folder).isDirectory()) {
                                folders.push(folder);
                                let found = false;
                                existingContainers.forEach(existingContainer => {
                                    if (existingContainer.name == folder) {
                                        found = true;
                                    }
                                });
                                if (!found) {
                                    actionsToTake++;
                                    fs.rename(hostedPath + folder + "/", tempPath + "noncorrelated-" + cryptotool.randomBytes(8).toString('hex') + "-" + folder + "/", function (err) {
                                        actionsToTake += -1;
                                        if (err) {
                                            Supervisor.emitter.emit('errorMovingUncorrelatedFolder', new Error(err.code));
                                            reject(err);
                                        } else {
                                            Supervisor.emitter.emit('movedUncorrelatedFolder');
                                        }
                                        if (actionsToTake <= 0) {
                                            resolve();
                                        }
                                    })
                                }
                            }
                        });

                        let existingContainerIds = [];
                        Supervisor.hostAuths.forEach(auth => {
                            existingContainerIds.push(auth.host.uuid);
                        });

                        existingContainers.forEach(containerInfo => {
                            if (!folders.includes(containerInfo.name) && !existingContainerIds.includes(containerInfo.name)) {
                                actionsToTake++;
                                Supervisor.docker.getContainer(containerInfo.id).remove({
                                    force: true
                                }, (err) => {
                                    actionsToTake += -1;
                                    if (err) {
                                        Supervisor.emitter.emit('errorRemovingUncorrelatedContainer');
                                        reject(err);
                                    } else {
                                        Supervisor.emitter.emit('removedUncorrelatedContainer');
                                    }
                                    if (actionsToTake <= 0) {
                                        resolve();
                                    }
                                });
                            }
                        });
                        if (actionsToTake <= 0) {
                            resolve();
                        }
                    };

                })
            } catch (err) {
                reject(err);
            }
        });
    }

}