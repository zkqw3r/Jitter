const ROOM_ID = location.pathname.split("/").pop();
const ws = new WebSocket(`wss://${location.host}/ws/${ROOM_ID}`);

const res = await fetch("/api/ice-config");
const iceConfig = await res.json();
const pc = new RTCPeerConnection(iceConfig);

const iceCandidateBuffer = [];
let remoteDescriptionSet = false;

const streamReady = navigator.mediaDevices
	.getUserMedia({ video: true, audio: true })
	.then((stream) => {
		document.getElementById("localVideo").srcObject = stream;
		stream.getTracks().forEach((t) => pc.addTrack(t, stream));
	});

pc.onicecandidate = ({ candidate }) => {
	if (candidate) {
		ws.send(JSON.stringify({ type: "ice", candidate }));
	}
};

pc.ontrack = ({ streams }) => {
	const video = document.getElementById("remoteVideo");
	video.srcObject = streams[0];
	streams[0].getVideoTracks().forEach((track) => {
		track.onmute = () => {
			video.style.display = "none";
			document.getElementById("remoteAvatar").classList.add("visible");
		};
		track.onunmute = () => {
			video.style.display = "block";
			document.getElementById("remoteAvatar").classList.remove("visible");
		};
	});
};

async function startCall() {
	await streamReady;
	const offer = await pc.createOffer();
	await pc.setLocalDescription(offer);
	ws.send(JSON.stringify({ type: "offer", sdp: offer }));
}

async function endCall() {
	const stream = document.getElementById("localVideo").srcObject;
	stream.getTracks().forEach((t) => t.stop());
	pc.close();
	ws.close();
	location.href = "/";
}

ws.onclose = (event) => {
	if (event.code === 4000) {
		document.body.innerHTML = `
            <div class="header">
                <h1 class="logo">Jitter</h1>
            </div>
            <div class="error-html-page">
                <h2>🚫 Комната заполнена</h2>
                <p>В этом звонке уже участвуют 2 человека</p>
                <a href="/" class="back-link">← На главную</a>
            </div>
        `;
	}
};

ws.onerror = () => {
	document.body.innerHTML = `
        <div class="header">
            <h1 class="logo">Jitter</h1>
        </div>
        <div class="error-html-page">
            <h2>❌ Комната не найдена</h2>
            <p>Возможно, ссылка устарела или комната была удалена</p>
            <a href="/" class="back-link">← На главную</a>
        </div>`;
};

ws.onmessage = async ({ data }) => {
	const msg = JSON.parse(data);

	if (msg.type === "offer") {
		if (pc.signalingState !== "stable") return;
		await streamReady;
		await pc.setRemoteDescription(msg.sdp);
		remoteDescriptionSet = true;
		for (const candidate of iceCandidateBuffer) {
			await pc.addIceCandidate(candidate);
		}
		iceCandidateBuffer.length = 0;
		const answer = await pc.createAnswer();
		await pc.setLocalDescription(answer);
		ws.send(JSON.stringify({ type: "answer", sdp: answer }));
	} else if (msg.type === "answer") {
		if (pc.signalingState !== "have-local-offer") return;
		await pc.setRemoteDescription(msg.sdp);
		remoteDescriptionSet = true;
		for (const candidate of iceCandidateBuffer) {
			await pc.addIceCandidate(candidate);
		}
		iceCandidateBuffer.length = 0;
	} else if (msg.type === "ice") {
		if (remoteDescriptionSet) {
			await pc.addIceCandidate(msg.candidate);
		} else {
			iceCandidateBuffer.push(msg.candidate);
		}
	} else if (msg.type === "cam-off") {
		document.getElementById("remoteVideo").style.display = "none";
		document.getElementById("remoteAvatar").classList.add("visible");
	} else if (msg.type === "cam-on") {
		document.getElementById("remoteVideo").style.display = "block";
		document.getElementById("remoteAvatar").classList.remove("visible");
	} else if (msg.type === "room-timeout") {
		endCall();
	} else if (msg.type === "peer-joined") {
		if (pc.signalingState === "stable") {
			startCall();
		}
	}
};

function toggleFullscreen(wrapId) {
	const wrap = document.getElementById(wrapId);
	if (!document.fullscreenElement) {
		wrap.requestFullscreen();
	} else {
		document.exitFullscreen();
	}
}

function toggleMic() {
	const track = document
		.getElementById("localVideo")
		.srcObject.getAudioTracks()[0];
	track.enabled = !track.enabled;
	const isMuted = !track.enabled;
	document.getElementById("micBtn").classList.toggle("off", isMuted);
	document.getElementById("micBtn").innerHTML =
		`<i data-lucide="${isMuted ? "mic-off" : "mic"}"></i>`;
	document.getElementById("micBtn").title = isMuted
		? "Включить микрофон"
		: "Выключить микрофон";
	lucide.createIcons();
}

function toggleCam() {
	const track = document
		.getElementById("localVideo")
		.srcObject.getVideoTracks()[0];
	track.enabled = !track.enabled;
	const isOff = !track.enabled;
	ws.send(JSON.stringify({ type: isOff ? "cam-off" : "cam-on" }));
	document.getElementById("camBtn").classList.toggle("off", isOff);
	document.getElementById("camBtn").innerHTML =
		`<i data-lucide="${isOff ? "video-off" : "video"}"></i>`;
	document.getElementById("camBtn").title = isOff
		? "Включить камеру"
		: "Выключить камеру";
	document.getElementById("localVideo").style.display = isOff
		? "none"
		: "block";
	document.getElementById("localAvatar").classList.toggle("visible", isOff);
	lucide.createIcons();
}

function toggleMute() {
	const video = document.getElementById("remoteVideo");
	video.muted = !video.muted;
	const isMuted = video.muted;
	document.getElementById("muteBtn").classList.toggle("off", isMuted);
	document.getElementById("muteBtn").innerHTML =
		`<i data-lucide="${isMuted ? "volume-x" : "volume-2"}"></i>`;
	document.getElementById("muteBtn").title = isMuted
		? "Включить звук"
		: "Выключить звук";
	lucide.createIcons();
}

function copyLink() {
	navigator.clipboard.writeText(location.href).then(() => {
		document.getElementById("copyBtn").innerHTML =
			'<i data-lucide="check"></i>';
		document.getElementById("copyBtn").title = "Скопировано!";
		lucide.createIcons();
		setTimeout(() => {
			document.getElementById("copyBtn").innerHTML =
				'<i data-lucide="link"></i>';
			document.getElementById("copyBtn").title = "Скопировать ссылку";
			lucide.createIcons();
		}, 2000);
	});
}

lucide.createIcons();
