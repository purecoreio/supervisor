const io = require('socket.io-client');
class SocketClient {

    public static socket;

    public setup() {
        SocketClient.socket = io.connect('https://socket.purecore.io/', { path: "/hosting" });

        /**
         * Authentication process:
         * 1. connect
         * 2. authenticate with hash
         * 3. set as host machine when authenticated
         */
        SocketClient.socket.on('connect', function () {
            Supervisor.emitter.emit('socketConnected')
            Supervisor.emitter.emit('socketAuthenticating')
            SocketClient.socket.emit('auth', Supervisor.hash)
        });
        SocketClient.socket.on('authenticated', function (data) {
            Supervisor.emitter.emit('socketAuthenticated')
            Supervisor.emitter.emit('socketHostRequest')
            SocketClient.socket.emit('host')
        });
        SocketClient.socket.on('hosting', function (data) {
            Supervisor.emitter.emit('socketHosting')
        });

        /** 
         *  Status Updates
         */
        SocketClient.socket.on('disconnect', function () { Supervisor.emitter.emit('socketDisconnected') });
    }

}