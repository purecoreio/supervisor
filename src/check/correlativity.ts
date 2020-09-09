const fs = require('fs');
const cryptotool = require("crypto");

class Correlativity {

    /**
     * Will move the folders on /etc/purecore/hosted/ to /etc/purecore/tmp/ when no purecore.io docker is matching that data
     */


    static basePath = "/etc/purecore/";
    static hostedPath = "/etc/purecore/hosted/";
    static tempPath = "/etc/purecore/tmp/";

    static checkFilesystem(existingContainers): Promise<void> {
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
                            })
                            fs.rename(Correlativity.hostedPath + folder + "/", Correlativity.tempPath + "noncorrelated-" + cryptotool.randomBytes(8).toString('hex') + "-" + folder + "/", function (err) {
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
                if (actionsToTake <= 0) {
                    resolve();
                }
            } catch (err) {
                reject(err);
            }
        })
    }

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

                        if (!fs.existsSync(Correlativity.basePath)) fs.mkdirSync(Correlativity.basePath)
                        if (!fs.existsSync(Correlativity.hostedPath)) fs.mkdirSync(Correlativity.hostedPath)
                        if (!fs.existsSync(Correlativity.tempPath)) fs.mkdirSync(Correlativity.tempPath)

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
                            })
                        } else {
                            existingContainers.forEach(containerInfo => {
                                if (!existingContainerIds.includes(containerInfo.name)) {
                                    // remove from existing containers (about to be deleted)
                                    sshdCheck.removeUser(containerInfo.name).catch(() => {
                                        //ignore
                                    })
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
                                        } else {
                                            Supervisor.emitter.emit('removedUncorrelatedContainer');
                                        }
                                        if (actionsToTake <= 0) {
                                            Correlativity.checkFilesystem(existingContainers).then(() => {
                                                resolve();
                                            }).catch((err) => {
                                                reject(err);
                                            })
                                        }
                                    });
                                }
                                if (actionsToTake == 0) {
                                    Correlativity.checkFilesystem(existingContainers).then(() => {
                                        resolve();
                                    }).catch((err) => {
                                        reject(err);
                                    })
                                }
                            });
                        }
                    };

                })
            } catch (err) {
                reject(err);
            }
        });
    }

}