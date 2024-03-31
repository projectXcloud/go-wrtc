
var ws = new WebSocket("ws://localhost:6080/ws");
var peerConnection = new RTCPeerConnection();

ws.onmessage = function (event) {
    // console.log("Received: " + event.data);
    // Parse the incoming message from JSON string to JavaScript object
    var message = JSON.parse(event.data);

    // Access the type and data of the message
    console.log("Message Type: " + message.type);
    console.log("Message Data: " + message.data);

    // Handle the message based on its type
    switch (message.type) {
        case "offer": {
            // console.log("Offer received:", message.data);

            // Assuming we have a peerConnection instance of RTCPeerConnection already created
            var offerDesc = new RTCSessionDescription(JSON.parse(message.data));
            // console.log(offerDesc, 1111)

            peerConnection.setRemoteDescription(offerDesc).then(function () {
                // Once the remote description is set, create an answer
                console.log(peerConnection.remoteDescription, 11112222)
                
                return peerConnection.createAnswer();
            })
                .then(function (answer) {
                    // Set the local description with the created answer
                    return peerConnection.setLocalDescription(answer);
                })
                .then(function () {
                    // Send the answer back to the server
                    console.log(peerConnection.localDescription, 22112222)
                    var answerMessage = {
                        type: "answer",
                        data: JSON.stringify(peerConnection.localDescription)
                    };

                    ws.send(JSON.stringify(answerMessage));
                })
                .catch(function (err) {
                    console.error("Error handling the offer: ", err);
                });

            // Listen for local ICE candidates on the peer connection
            peerConnection.onicecandidate = function (event) {
                if (event.candidate) {
                    console.log("New ICE candidate:", event.candidate);

                    // Send the ICE candidate to the remote peer
                    var candidateMessage = {
                        type: "candidate",
                        data: JSON.stringify(event.candidate)
                    };

                    ws.send(JSON.stringify(candidateMessage));
                }
            };
            break;
        }
        case "candidate": {
            console.log("ICE candidate received:", message.data);
        
            function addCandidate() {
                var candidate = new RTCIceCandidate(JSON.parse(message.data));
                peerConnection.addIceCandidate(candidate).catch(function (err) {
                    console.error("Error adding received ICE candidate:", err);
                });
            }
        
            // Check if both local and remote descriptions are set
            if (peerConnection.signalingState === "stable" || peerConnection.signalingState === "have-local-offer") {
                addCandidate();
            } else {
                console.log("Waiting for local and remote descriptions to be set before adding candidates");
        
                // Option 1: Queue the candidate and add it later
                // This would require implementing a queue to hold candidates until they can be added.
        
                // Option 2: Use an event listener to wait for the descriptions to be set
                // This example will add the candidate once the signaling state changes to stable.
                // Note: You need to remove this listener when appropriate to avoid leaks.
                var signalingStateListener = function() {
                    if (peerConnection.signalingState === "stable") {
                        addCandidate();
                        peerConnection.removeEventListener("signalingstatechange", signalingStateListener);
                    }
                };
        
                peerConnection.addEventListener("signalingstatechange", signalingStateListener);
            }
            break;
        }
        
        case "answer":
            // Handle answer
            console.log("Answer received:", message.data);
            // Process the answer here
            break;
        case "initiation":
            // Handle initiation
            console.log("Initiation message received:", message.data);
            // Process initiation here
            break;
        // Add more cases as needed for other message types
        default:
            console.log("Unknown message type:", message.type);
    }
};
ws.onopen = function (event) {
    ws.send(JSON.stringify({
        type: "Initiation",
        data: "Initiation of WebRTC"
    }));
};