const SSHConfig = require('ssh-config')
class sshdCheck {

    public static sshdConfigPath = "/etc/ssh/sshd_config";

    public static getCurrentConfig() {
        try {
            let rawdata = fs.readFileSync(sshdCheck.sshdConfigPath, 'utf8');
            console.log(rawdata);
            const config = SSHConfig.parse(rawdata)
            return config;
        } catch (error) {
            Supervisor.emitter.emit('sshdParseError', error);
        }
    }

    public static getNewConfig() {
        let config: Array<any> = sshdCheck.getCurrentConfig();
        let addSubsystem = false;
        let chrootRuleFound = false;
        for (let index = 0; index < config.length; index++) {
            let element = config[index];
            if (element.param == 'Subsystem' && element.type != 2) {
                if (element.value != 'sftp\tinternal-sftp') {
                    element.type = 2;
                    element.content = `#${element.param}${element.value} [before purecore installation]`
                    delete element.param;
                    delete element.value;
                    delete element.separator;
                    config[index] = element;
                    addSubsystem = true;
                }
            }
            if (element.param == 'Match') {
                console.log(element);
            }
        }
        if (addSubsystem) {
            config.push({
                type: 1,
                param: 'Subsystem',
                separator: '\t',
                value: 'sftp\tinternal-sftp',
                before: '',
                after: '\n\n'
            })
        }
        return config;
    }

}