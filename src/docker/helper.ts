class DockerHelper {

    public static hostingFolder = "/opt/purecore/hosted/"

    public static createContainer(hostRequest): Promise<void> {

        return new Promise(function (resolve, reject) {
            Supervisor.emitter.emit('creatingContainer');

            try {
                Supervisor.docker.createContainer({
                    Image: hostRequest.image, name: 'core-' + hostRequest.uuid, Env: [
                        "EULA=true",
                    ], HostConfig: {
                        PortBindings: { '25565/tcp': [{ HostPort: String(hostRequest.port) }] },
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