const ws = new WebSocket("ws://localhost:9091/ws");

let peerConnection = null;
let pendingIceCandidates = [];
let pendingRemoteIceCandidates = [];

ws.onopen = () => console.log("connected");
ws.onclose = () => console.log("disconnected");
ws.onerror = (event) => console.log("ws error:", event);

ws.onmessage = async (event) => {
  console.log(event);

  const message = JSON.parse(event.data);
  console.log("message recieved on socket-" + message.type);

  switch (message.type) {
    case "answer":
      console.log("processing answer");
      try {
        await peerConnection.setRemoteDescription(message.data);
        console.log("remote description set on answer message");
      } catch (error) {
        console.error("Failed to set remote description:", error);
      }

      for (const candidate of pendingIceCandidates) {
        const iceMessage = {
          type: "ice",
          data: candidate,
        };
        ws.send(JSON.stringify(iceMessage));
        console.log("ice message send from client");
      }
      pendingIceCandidates = [];
      for (const remoteCandidate of pendingRemoteIceCandidates) {
        console.log("*********************");
        console.log(remoteCandidate);
        console.log("*********************");

        try {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(remoteCandidate),
          );
          console.log("remote ICE candidate added on answer message");
        } catch (iceErr) {
          console.warn("Failed to add ICE candidate:", iceErr);
        }
      }
      pendingRemoteIceCandidates = [];
      break;

    case "ice":
      console.log("processing ice");
      console.log(message);
      console.log("~~~~~~~~~~~");
      console.log(peerConnection.remoteDescription);
      console.log("~~~~~~~~~~~");
      // const candidate = JSON.parse(message.data);
      try {
        if (peerConnection.remoteDescription) {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(message.data),
          );
          console.log("remote ICE candidate added immediately");
        } else {
          pendingRemoteIceCandidates.push(message.data);
          console.log("remote ICE candidate buffered");
        }
      } catch (iceErr) {
        console.log("Failed to add ICE candidate:", iceErr);
      }
      break;
    default:
      console.log("undefined case");
      break;
  }
};

const localCamVideoEl = document.getElementById("local-cam-video");
const remoteCamVideoEl = document.getElementById("remote-cam-video");
const micSelect = document.getElementById("availableMicrophones");
const cameraSelect = document.getElementById("availableCameras");
const startButton = document.getElementById("start");
const remoteCamVideoSection = document.getElementById("remoteCamVideoSection");
const audioEl = document.getElementById("audio");

peerConnection = new RTCPeerConnection({
  ice: [],
});
console.log("peer connection created");

const audioTransceiver = peerConnection.addTransceiver("audio", {
  direction: "sendrecv",
});

const cameraTransceiver = peerConnection.addTransceiver("video", {
  direction: "sendrecv",
});

peerConnection.onicecandidate = async (e) => {
  if (e.candidate == null) return;
  console.log("------------ice generated at client------------");
  console.log(JSON.stringify(e.candidate));
  console.log("------------ice generated at client------------");
  console.log("onicecandidate:" + e);
  if (peerConnection.remoteDescription) {
    const iceMessage = {
      type: "ice",
      data: e.candidate,
    };
    ws.send(JSON.stringify(iceMessage));
    console.log("ice message send from client");
  } else {
    console.log("buffering ice candidate from ice generation");
    pendingIceCandidates.push(e.candidate);
  }
};

peerConnection.onconnectionstatechange = () => {
  console.log("connection state change:", peerConnection.connectionState);
};

peerConnection.ontrack = (e) => {
  console.log("Track received:", e.track.kind);
  console.log(e);

  if (e.transceiver.mid === "0") {
    console.log("+++++++++++++audio transceiver++++++++++++++");
    const stream = new MediaStream();
    stream.addTrack(e.track);
    console.log("`````````````````````````");
    console.log(stream);
    console.log(e.streams[0]);
    console.log("`````````````````````````");

    audioEl.srcObject = stream;
    audioEl.muted = false;
    audioEl.play().catch((err) => {
      console.log("remote audio playing err");
      console.log(err);
    });
    console.log("audio stream set");
  }

  if (e.transceiver.mid === "1") {
    console.log("+++++++++++++camera transceiver++++++++++++++");
    const stream = new MediaStream();
    stream.addTrack(e.track);
    console.log("`````````````````````````");
    console.log(stream);
    console.log(e.streams[0]);
    console.log("`````````````````````````");
    remoteCamVideoEl.srcObject = stream;
    remoteCamVideoEl.play().catch((err) => {
      console.log("remote video playing err");
      console.log(err);
    });
    console.log("camvideo stream set");
  }
};

// Get media devices
async function getDevices(kind) {
  const devices = await navigator.mediaDevices.enumerateDevices();
  return devices.filter((d) => d.kind === kind);
}

// Populate device dropdowns
async function updateDeviceList() {
  const cameras = await getDevices("videoinput");
  const mics = await getDevices("audioinput");

  console.log(cameras);
  console.log(mics);

  cameraSelect.innerHTML = '<option value="">Select Camera</option>';
  cameras.forEach((cam) => {
    cameraSelect.appendChild(
      Object.assign(document.createElement("option"), {
        value: cam.deviceId,
        textContent: cam.label || "Camera",
      }),
    );
  });

  micSelect.innerHTML = '<option value="">Select Microphone</option>';
  mics.forEach((mic) => {
    micSelect.appendChild(
      Object.assign(document.createElement("option"), {
        value: mic.deviceId,
        textContent: mic.label || "Microphone",
      }),
    );
  });
}

updateDeviceList();

document.getElementById("start").onclick = async () => {
  const cameraId = cameraSelect.value;
  const micId = micSelect.value;

  if (!cameraId || !micId) {
    alert("Select both camera and microphone");
    return;
  }
  console.log("Starting call...");

  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        deviceId: { exact: micId },
        echoCancellation: true,
        noiseSuppression: true,
      },
      video: { deviceId: { exact: cameraId } },
    });

    console.log("Local media stream obtained:", stream);

    const videoTracks = stream.getVideoTracks();
    const audioTracks = stream.getAudioTracks();
    console.log(
      "Video tracks:",
      videoTracks.length,
      "Audio tracks:",
      audioTracks.length,
    );

    audioTransceiver.sender.replaceTrack(audioTracks[0]);
    cameraTransceiver.sender.replaceTrack(videoTracks[0]);

    console.log("replaced tracks");

    const offer = await peerConnection.createOffer();
    console.log("offer created:");
    console.log(offer);

    await peerConnection.setLocalDescription(offer);
    console.log("set local desc");

    localCamVideoEl.srcObject = stream;
    await localCamVideoEl.play().catch((err) => {
      console.log("local video playing err");
      console.log(err);
    });
    console.log("awaited");

    const offerMessage = {
      type: "offer",
      data: peerConnection.localDescription,
    };
    ws.send(JSON.stringify(offerMessage));
    console.log("ws message send");
  } catch (error) {
    console.error("Error starting call:", error);
    alert("error");
  }
};
