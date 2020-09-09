const SSHConfig = require('ssh-config')
const linuxUser = require('linux-sys-user');
const chown = require('chown');

class sshdCheck {

    public static sshdConfigPath = "/etc/ssh/sshd_config";

    public static getCurrentConfig() {
        try {
            let rawdata = fs.readFileSync(sshdCheck.sshdConfigPath, 'utf8');
            const config = SSHConfig.parse(rawdata)
            return config;
        } catch (error) {
            Supervisor.emitter.emit('sshdParseError', error);
        }
    }

    public static getNewConfig() {
        let config = sshdCheck.getCurrentConfig();
        let addSubsystem = false;
        let chrootRuleFound = false;
        for (let index = 0; index < config.length; index++) {
            let element = config[index];
            if (element.param == 'Subsystem' && element.type != 2) {
                if (!element.value.includes('sftp') || !element.value.includes('internal-sftp')) {
                    element.type = 2;
                    element.content = `#${element.param}${element.value} [before purecore installation]`
                    delete element.param;
                    delete element.value;
                    delete element.separator;
                    config[index] = element;
                    addSubsystem = true;
                }
            }
            if (element.param == 'Match' && element.type != 2) {
                if (element.value == 'Group purecore') chrootRuleFound = true;
            }
        }
        if (addSubsystem) {
            Supervisor.emitter.emit('sshdPendingSubsystem');
            config.push({
                type: 1,
                param: 'Subsystem',
                separator: '\t',
                value: 'sftp\tinternal-sftp',
                before: '',
                after: '\n\n'
            })
        }

        if (!chrootRuleFound) {
            Supervisor.emitter.emit('sshdPendingChroot');
            config.append(
                {
                    Match: 'Group purecore',
                    ChrootDirectory: '/etc/purecore/hosted/%u',
                    ForceCommand: 'internal-sftp'
                }
            )
        }

        return SSHConfig.stringify(config);
    }

    public static async createGroupIfNeeded(): Promise<void> {
        return new Promise(function (resolve, reject) {
            linuxUser.getGroupInfo('purecore', function (err, data) {
                if (err || data == null) {
                    Supervisor.emitter.emit('creatingGroup');
                    linuxUser.addGroup('purecore', function (err, data) {
                        if (err) {
                            Supervisor.emitter.emit('errorCreatingGroup');
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
        console.log("creating user");
        return new Promise(function (resolve, reject) {
            sshdCheck.createGroupIfNeeded().then(() => {
                console.log("group present");
                Supervisor.emitter.emit('creatingUser');
                const userPath = Correlativity.hostedPath + hostAuth.host.uuid;
                const dataPath = userPath + "/data";
                if (!fs.existsSync(userPath)) fs.mkdirSync(userPath)
                if (!fs.existsSync(dataPath)) fs.mkdirSync(dataPath)
                console.log(hostAuth.host)
                linuxUser.addUser({ username: hostAuth.host.uuid, create_home: true, home_dir: dataPath, shell: null }, function (err, user) {
                    if (err) {
                        console.log("error creating user " + err.message);
                        Supervisor.emitter.emit('errorCreatingUser');
                        reject();
                    }
                    chown(dataPath, hostAuth.host.uuid, 'purecore')
                        .then(() => {
                            console.log("chowned");
                            Supervisor.emitter.emit('createdUser');
                            Supervisor.emitter.emit('addingUserToGroup');
                            linuxUser.addUserToGroup(hostAuth.host.uuid, 'purecore', function (err, user) {
                                if (err) {
                                    console.log("error adding to group")
                                    Supervisor.emitter.emit('errorAddingUserToGroup');
                                    reject();
                                }
                                console.log("added to group")
                                Supervisor.emitter.emit('addedUserToGroup');
                                Supervisor.emitter.emit('settingUserPassword');
                                linuxUser.setPassword(hostAuth.host.uuid, hostAuth.hash, function (err, user) {
                                    if (err) {
                                        console.log("error setting password")
                                        Supervisor.emitter.emit('errorSettingUserPassword');
                                        reject();
                                    }
                                    console.log("set password")
                                    Supervisor.emitter.emit('setUserPassword');
                                    resolve();
                                });
                            });
                        })
                        .catch((err) => {
                            console.log("error chowning");
                            Supervisor.emitter.emit('errorChowningUser', err);
                        });
                });
            }).catch((err) => {
                console.log("error creating group " + err.message);
                reject();
            })
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

    public static applyConfig(): Promise<void> {
        return new Promise(function (resolve, reject) {
            sshdCheck.createGroupIfNeeded().then(() => {
                let newConfig = sshdCheck.getNewConfig();
                if (SSHConfig.stringify(sshdCheck.getCurrentConfig()) != newConfig) {
                    Supervisor.emitter.emit('sshdConfigurationChanging');
                    fs.writeFile(sshdCheck.sshdConfigPath, newConfig, 'utf8', function (err) {
                        if (err) {
                            Supervisor.emitter.emit('sshdConfigurationChangeError');
                            reject();
                        }
                        Supervisor.emitter.emit('sshdConfigurationChanged');
                        resolve();
                    });
                } else {
                    resolve();
                }
            }).catch(() => {
                Supervisor.emitter.emit('errorCreatingGroup');
            })
        })
    }

}