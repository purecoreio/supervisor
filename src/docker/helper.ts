const { PassThrough } = require('stream')
const linuxUser = require('linux-sys-user');
const chroot = require('chroot');

class DockerHelper {

    public static hostingFolder = "/etc/purecore/hosted/"

    public static async getContainer(host): Promise<any> {
        return new Promise(function (resolve, reject) {
            Supervisor.docker.listContainers(function (err, containers) {
                for (let index = 0; index < containers.length; index++) {
                    const element = containers[index];
                    for (var name of element.Names) {
                        name = String(name);
                        const prefix = "core-";
                        if (name.substr(0, 1) == '/') name = name.substr(1, name.length - 1);
                        if (name.includes(prefix) && name.substr(0, prefix.length) == prefix && name.substr(prefix.length, name.length - prefix.length) == host.uuid) {
                            resolve(Supervisor.docker.getContainer(element.Id));
                            break;
                        }
                    }
                }
            });
        });
    }

    public static async removeUser(username): Promise<void> {
        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('removingUser');
            if (typeof username == 'string' && username.length == 16) {
                linuxUser.removeUser(username, function (err, data) {
                    if (err || data == null) {
                        Supervisor.emitter.emit('errorRemovingUser');
                        reject();
                    } else {
                        Supervisor.emitter.emit('removedUser');
                        resolve();
                    }
                });
            } else {
                reject();
            }
        })
    }

    public static async createGroupIfNeeded(): Promise<void> {
        return new Promise(function (resolve, reject) {
            linuxUser.getGroupInfo('purecore', function (err, data) {
                if (err || data == null) {
                    Supervisor.emitter.emit('creatingGroup');
                    linuxUser.addGroup('purecore', function (err, data) {
                        if (err) {
                            reject();
                        } else {
                            Supervisor.emitter.emit('createdGroup');
                            resolve();
                        }
                    })
                } else {
                    resolve();
                }
            })
        });
    }

    public static async createUser(hostAuth): Promise<void> {
        return new Promise(function (resolve, reject) {
            DockerHelper.createGroupIfNeeded().then(() => {
                Supervisor.emitter.emit('creatingUser');
                linuxUser.addUser({ username: hostAuth.host.uuid, create_home: false, shell: null }, function (err, user) {
                    if (err) {
                        Supervisor.emitter.emit('errorCreatingUser');
                        reject();
                    }
                    Supervisor.emitter.emit('createdUser');
                    Supervisor.emitter.emit('addingUserToGroup');
                    linuxUser.addUserToGroup(hostAuth.host.uuid, 'purecore', function (err, user) {
                        if (err) {
                            Supervisor.emitter.emit('errorAddingUserToGroup');
                            reject();
                        }
                        Supervisor.emitter.emit('addedUserToGroup');
                        Supervisor.emitter.emit('settingUserPassword');
                        linuxUser.setPassword(hostAuth.host.uuid, hostAuth.hash, function (err, user) {
                            if (err) {
                                Supervisor.emitter.emit('errorSettingUserPassword');
                                reject();
                            }
                            Supervisor.emitter.emit('setUserPassword');
                            Supervisor.emitter.emit('settingUserRoot');
                            try {
                                chroot(`${Correlativity.hostedPath}${hostAuth.host.uuid}/`, hostAuth.host.uuid);
                                Supervisor.emitter.emit('setUserRoot');
                                resolve();
                            } catch (error) {
                                Supervisor.emitter.emit('errorSettingUserRoot');
                                reject();
                            }
                        });
                    });
                });
            }).catch((err) => {
                Supervisor.emitter.emit('errorCreatingGroup');
                reject();
            })
        });
    }

    public static async getLogStream(container): Promise<any> {
        return new Promise(function (resolve, reject) {
            const logStream = new PassThrough();
            if (container != null) {
                container.logs({
                    follow: true,
                    stdout: true,
                    stderr: true
                }, function (err, stream) {
                    container.modem.demuxStream(stream, logStream, logStream)
                    stream.on('end', function () {
                        logStream.end('!stop');
                    })
                    resolve(logStream);
                })
            } else {
                throw new Error("Unknown container");

            }
        });
    }

    public static actuallyCreateContainer(retrypquota, opts): Promise<any> {
        let promise: Promise<any> = new Promise(function (resolve, reject) {
            Supervisor.docker.createContainer(opts).then((container) => {
                Supervisor.emitter.emit('createdContainer');
                Supervisor.emitter.emit('startingNewContainer');
                container.start().then(() => {
                    Supervisor.emitter.emit('startedNewContainer');
                    resolve(null);
                })
            }).catch((error) => {
                if (retrypquota && error.message.includes('pquota')) {
                    ConsoleUtil.setLoading(false, "Creating container with no size limit, please, use the overlay2 storage driver, back it with extfs, enable d_type and make sure pquota is available (probably your issue, read more here: https://stackoverflow.com/a/57248363/7280257)", false, true, false);
                    delete opts.HostConfig.StorageOpt;
                    let newPromise = DockerHelper.actuallyCreateContainer(false, opts);
                    resolve(newPromise)
                } else {
                    reject(error);
                }
            })
        })
        return promise;
    }

    public static createContainer(authRequest): Promise<void> {

        authRequest = Supervisor.machine.core.getHostingManager().getHostAuth().fromObject(authRequest);

        if (!Supervisor.hostAuths.includes(authRequest)) {
            Supervisor.hostAuths.push(authRequest);
        }

        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('creatingContainer');

            const basePath = "/etc/purecore/";
            const hostedPath = basePath + "hosted/";

            let opts: any = {
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
                        `${hostedPath}/${authRequest.host.uuid}:/data`
                    ],
                    NanoCpus: authRequest.host.template.cores * 10 ^ 9
                },
            }

            try {
                DockerHelper.actuallyCreateContainer(true, opts).then((res) => {
                    if (res == null) {
                        Supervisor.emitter.emit('registeringUser');
                        DockerHelper.createUser(authRequest).then(() => {
                            Supervisor.emitter.emit('registeredUser');
                            resolve();
                        }).catch(() => {
                            Supervisor.emitter.emit('errorUserRegistration');
                        })
                    } else {
                        res.then(() => {
                            Supervisor.emitter.emit('registeringUser');
                            DockerHelper.createUser(authRequest).then(() => {
                                Supervisor.emitter.emit('registeredUser');
                                resolve();
                            }).catch(() => {
                                Supervisor.emitter.emit('errorUserRegistration');
                            })
                        }).catch((err) => {
                            Supervisor.emitter.emit('containerCreationError', err); reject();
                        })
                    }
                }).catch((err) => {
                    Supervisor.emitter.emit('containerCreationError', err); reject();
                })
            } catch (error) {
                Supervisor.emitter.emit('containerCreationError', error); reject();
            }
        });
    }

}