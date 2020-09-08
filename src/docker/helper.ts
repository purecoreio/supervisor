const { PassThrough } = require('stream')
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

    public static createContainer(authRequest): Promise<void> {

        authRequest = Supervisor.machine.core.getHostingManager().getHostAuth().fromObject(authRequest);

        if (!Supervisor.hostAuths.includes(authRequest)) {
            Supervisor.hostAuths.push(authRequest);
        }

        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('creatingContainer');

            try {
                Supervisor.docker.createContainer({
                    Image: authRequest.host.image, name: 'core-' + authRequest.host.uuid, Env: [
                        "EULA=true",
                    ], HostConfig: {
                        PortBindings: { '25565/tcp': [{ HostPort: String(authRequest.host.port) }] },
                    },
                }).then((container) => {
                    Supervisor.emitter.emit('createdContainer');
                    Supervisor.emitter.emit('startingNewContainer');
                    container.start().then(() => {
                        Supervisor.emitter.emit('startedNewContainer');
                        resolve();
                    })

                }).catch((error) => {
                    Supervisor.emitter.emit('containerCreationError', error); reject();
                })
            } catch (error) {
                Supervisor.emitter.emit('containerCreationError', error); reject();
            }
        });
    }

}