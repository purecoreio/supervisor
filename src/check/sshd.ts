const SSHConfig = require('ssh-config')
const linuxUser = require('linux-sys-user');
const chownr = require('chownr');

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

    public static async createGroupIfNeeded(): Promise<any> {
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
                            resolve(data);
                        }
                    })
                } else {
                    resolve(data);
                }
            })
        });
    }


    public static async createUser(hostAuth): Promise<any> {
        return new Promise(function (resolve, reject) {
            sshdCheck.createGroupIfNeeded().then((g) => {
                Supervisor.emitter.emit('creatingUser');
                const userPath = Correlativity.hostedPath + hostAuth.host.uuid;
                const dataPath = userPath + "/data";
                if (!fs.existsSync(userPath)) fs.mkdirSync(userPath)
                if (!fs.existsSync(dataPath)) fs.mkdirSync(dataPath)
                linuxUser.getUserInfo(`u${hostAuth.host.uuid}`, function (err, user) {
                    let alreadyPresent = true;
                    if (err || user == null) {
                        alreadyPresent = false;
                    }
                    if (!alreadyPresent) {
                        linuxUser.addUser({ username: `u${hostAuth.host.uuid}`, create_home: true, home_dir: dataPath, shell: null }, function (err, user) {
                            if (err) {
                                Supervisor.emitter.emit('errorCreatingUser');
                                reject(err);
                            }
                            Supervisor.emitter.emit('createdUser');
                            Supervisor.emitter.emit('chowningUser');
                            chownr(dataPath, user.uid, g.gid, function (err, data) {
                                if (err) {
                                    Supervisor.emitter.emit('errorChowningUser', err);
                                }
                                Supervisor.emitter.emit('chownedUser');
                                Supervisor.emitter.emit('addingUserToGroup');
                                linuxUser.addUserToGroup(`u${hostAuth.host.uuid}`, 'purecore', function (err, data) {
                                    if (err) {
                                        Supervisor.emitter.emit('errorAddingUserToGroup');
                                        reject(err);
                                    }
                                    Supervisor.emitter.emit('addedUserToGroup');
                                    Supervisor.emitter.emit('settingUserPassword');
                                    linuxUser.setPassword(`u${hostAuth.host.uuid}`, hostAuth.hash, function (err, data) {
                                        if (err) {
                                            Supervisor.emitter.emit('errorSettingUserPassword');
                                            reject(err);
                                        }
                                        Supervisor.emitter.emit('setUserPassword');
                                        resolve({
                                            user: user,
                                            group: g
                                        });
                                    });
                                });
                            })
                        });
                    } else {
                        Supervisor.emitter.emit('chowningUser');
                        chownr(dataPath, user.uid, g.gid, function (err, data) {
                            if (err) {
                                Supervisor.emitter.emit('errorChowningUser', err);
                                reject(err);
                            }
                            Supervisor.emitter.emit('chownedUser');
                            resolve({
                                user: user,
                                group: g
                            });
                        })
                    }
                })
            }).catch((err) => {
                reject(err);
            })
        });
    }

    public static async removeUser(username): Promise<void> {
        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('removingUser');
            if (typeof username == 'string') {
                let char = username.substring(0, 1);
                if (char == "u" && username.length == 17) {
                    username = username.substring(1);
                }
                if (typeof username == 'string' && username.length == 16) {
                    linuxUser.removeUser('u' + username, function (err, data) {
                        if (err || data == null) {
                            Supervisor.emitter.emit('errorRemovingUser', err);
                            reject(err);
                        } else {
                            Supervisor.emitter.emit('removedUser', data);
                            resolve();
                        }
                    });
                } else {
                    const error = new Error("Invalid username value provided (invalid type or length)");
                    Supervisor.emitter.emit('errorRemovingUser', error);
                    reject(error);
                }
            } else {
                const error = new Error("Invalid username value provided")
                Supervisor.emitter.emit('errorRemovingUser', error);
                reject(error);
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