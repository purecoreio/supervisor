class DockerLogger {

    public static health: Array<HealthLog> = new Array<HealthLog>();

    public static getHealthEmitter(hostid: string) {
        try {
            let healthIndex = null;
            for (let index = 0; index < DockerLogger.health.length; index++) {
                const element = DockerLogger.health[index];
                if (element.host == hostid) {
                    healthIndex = index;
                }
            }
            if (healthIndex == null) {
                throw new Error("No matching hosts");
            } else {
                return DockerLogger.health[healthIndex].emitter;
            }
        } catch (error) {
            // ignore
        }
    }

    public static addLog(hostid: string, log) {
        try {
            let healthIndex = null;
            for (let index = 0; index < DockerLogger.health.length; index++) {
                const element = DockerLogger.health[index];
                if (element.host == hostid) {
                    healthIndex = index;
                }
            }
            if (healthIndex == null) {
                throw new Error("No matching hosts");
            } else {
                DockerLogger.health[healthIndex].pushLog(log);
            }
        } catch (error) {
            // ignore
        }
    }

    public static setHealthLog(hostid: string) {
        for (let index = 0; index < DockerLogger.health.length; index++) {
            const element = DockerLogger.health[index];
            if (element.host == hostid) {
                // delete existing host logs in order to prevent duplicates
                delete DockerLogger.health[index];
                break;
            }
        }
        DockerLogger.health.push(new HealthLog(hostid, new Array<any>()));
    }

    public static pushAllExistingContainers(): Promise<void> {
        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('startingHealthLogger');
            Supervisor.docker.listContainers({ all: true }).then((containers) => {
                let existingContainers = new Array<any>();
                containers.forEach(function (containerInfo) {
                    for (var name of containerInfo.Names) {
                        name = String(name);
                        const prefix = "core-";
                        if (name.substr(0, 1) == '/') name = name.substr(1, name.length - 1);
                        if (name.includes(prefix) && name.substr(0, prefix.length) == prefix) {
                            existingContainers.push({
                                host: name.substr(prefix.length, name.length - prefix.length),
                                container: containerInfo.Id
                            })
                        }
                    }
                });
                let todo = existingContainers.length;
                if (todo <= 0) {
                    Supervisor.emitter.emit('startedHealthLogger');
                    resolve();
                } else {
                    existingContainers.forEach(containerInfo => {
                        DockerLogger.startLogging(containerInfo.host);
                        todo += -1;
                        if (todo <= 0) {
                            Supervisor.emitter.emit('startedHealthLogger');
                            resolve();
                        }
                    });
                }
            }).catch((err) => {
                Supervisor.emitter.emit('errorStartingHealthLogger');
                reject(new Error("error listing existing containers: " + err.message))
            })
        })
    }

    public static startLogging(hostid: string): Promise<void> {
        return new Promise(function (resolve, reject) {
            Supervisor.docker.listContainers({ all: true }).then((containers) => {
                let container = null;
                containers.forEach(function (containerInfo) {
                    for (var name of containerInfo.Names) {
                        name = String(name);
                        const prefix = "core-";
                        if (name.substr(0, 1) == '/') name = name.substr(1, name.length - 1);
                        if (name.includes(prefix) && name.substr(0, prefix.length) == prefix) {
                            if (name.substr(prefix.length, name.length - prefix.length) == hostid) {
                                container = containerInfo.Id;
                                break;
                            }
                        }
                    }
                });
                if (container == null) {
                    reject(new Error("no attached container for " + hostid));
                } else {
                    Supervisor.docker.getContainer(container).then((actualContainer) => {
                        actualContainer.stats({ stream: true }).then((statStream) => {
                            DockerLogger.setHealthLog(hostid);
                            statStream.on('data', (stat) => {
                                DockerLogger.addLog(hostid, stat);
                            });
                        })
                    }).catch((err) => {
                        reject(new Error("error while getting the actual container: " + err.message))
                    })
                }
            }).catch((err) => {
                reject(new Error("error listing existing containers: " + err.message))
            })
        })
    }

}