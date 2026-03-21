const ROOM_ID = location.pathname.split("/").pop();
const ws = new WebSocket(`wss://${location.host}/ws/${ROOM_ID}`);

const res = await fetch("/api/ice-config");
const iceConfig = await res.json();
const pc = new RTCPeerConnection(iceConfig);

const iceCandidateBuffer = [];
let remoteDescriptionSet = false;

let currentFacingMode = "user";

async function checkAndShowFlipButton() {
	const isTouch = navigator.maxTouchPoints > 0 || "ontouchstart" in window;
	if (!isTouch) return;

	try {
		const devices = await navigator.mediaDevices.enumerateDevices();
		const videoInputs = devices.filter((d) => d.kind === "videoinput");

		if (videoInputs.length > 1) {
			document.querySelector(".camera-flip").style.display = "block";
		}
	} catch (err) {
		console.warn("enumerateDevices failed:", err);
	}
}
async function flipCamera() {
	const videoSender = pc.getSenders().find((s) => s.track?.kind === "video");
	if (!videoSender) return;

	const localVideo = document.getElementById("localVideo");
	const nextFacingMode =
		currentFacingMode === "user" ? "environment" : "user";

	const audioTracks = localVideo.srcObject
		? localVideo.srcObject.getAudioTracks()
		: [];
	if (localVideo.srcObject) {
		localVideo.srcObject.getVideoTracks().forEach((t) => t.stop());
	}
	videoSender.track.stop();

	localVideo.srcObject = null;

	try {
		const newStream = await navigator.mediaDevices.getUserMedia({
			video: { facingMode: { ideal: nextFacingMode } },
			audio: false,
		});

		const newTrack = newStream.getVideoTracks()[0];

		await videoSender.replaceTrack(newTrack);

		const combinedStream = new MediaStream([newTrack, ...audioTracks]);
		localVideo.srcObject = combinedStream;

		currentFacingMode = nextFacingMode;
		localVideo.style.transform =
			currentFacingMode === "user" ? "scaleX(-1)" : "scaleX(1)";
	} catch (err) {
		console.error("flipCamera error:", err);

		try {
			const fallbackStream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: { ideal: currentFacingMode } },
				audio: false,
			});
			const fallbackTrack = fallbackStream.getVideoTracks()[0];
			await videoSender.replaceTrack(fallbackTrack);
			localVideo.srcObject = new MediaStream([
				fallbackTrack,
				...audioTracks,
			]);
		} catch (fallbackErr) {
			console.error("flipCamera fallback failed:", fallbackErr);
		}
	}
}

const streamReady = navigator.mediaDevices
	.getUserMedia({ video: { facingMode: "user" }, audio: true })
	.then((stream) => {
		document.getElementById("localVideo").srcObject = stream;
		stream.getTracks().forEach((t) => pc.addTrack(t, stream));
		checkAndShowFlipButton();
	})
	.catch((err) => {
		console.error("Ошибка камеры/микрофона:", err);
		return navigator.mediaDevices
			.getUserMedia({ video: { facingMode: "user" }, audio: false })
			.then((stream) => {
				document.getElementById("localVideo").srcObject = stream;
				stream.getTracks().forEach((t) => pc.addTrack(t, stream));
				alert("Микрофон недоступен, работаем только с видео");
			})
			.catch((err2) => {
				alert(
					"Не удалось получить доступ к камере/микрофону: " +
						err2.message,
				);
			});
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
	try {
		const msg = JSON.parse(data);

		if (msg.type === "offer") {
			if (pc.signalingState !== "stable") return;
			await pc.setRemoteDescription(msg.sdp);
			remoteDescriptionSet = true;
			for (const candidate of iceCandidateBuffer) {
				await pc.addIceCandidate(candidate);
			}
			iceCandidateBuffer.length = 0;
			await streamReady;
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
				await startCall();
			}
		}
	} catch (err) {
		console.error("ws.onmessage error:", err);
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
	const stream = document.getElementById("localVideo").srcObject;
	if (!stream) return;

	const tracks = stream.getAudioTracks();
	if (tracks.length === 0) {
		alert("Микрофон не подключен");
		return;
	}

	const track = tracks[0];
	track.enabled = !track.enabled;
	const isMuted = !track.enabled;

	const btn = document.getElementById("micBtn");
	btn.classList.toggle("off", isMuted);
	btn.innerHTML = `<i data-lucide="${isMuted ? "mic-off" : "mic"}"></i>`;
	btn.title = isMuted ? "Включить микрофон" : "Выключить микрофон";
	window.lucide?.createIcons();
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
	window.lucide?.createIcons();
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
	window.lucide?.createIcons();
}

function copyLink() {
	navigator.clipboard.writeText(location.href).then(() => {
		document.getElementById("copyBtn").innerHTML =
			'<i data-lucide="check"></i>';
		document.getElementById("copyBtn").title = "Скопировано!";
		window.lucide?.createIcons();
		setTimeout(() => {
			document.getElementById("copyBtn").innerHTML =
				'<i data-lucide="link"></i>';
			document.getElementById("copyBtn").title = "Скопировать ссылку";
			window.lucide?.createIcons();
		}, 2000);
	});
}

window.lucide?.createIcons();
document.getElementById("micBtn").addEventListener("click", toggleMic);
document.getElementById("camBtn").addEventListener("click", toggleCam);
document.getElementById("muteBtn").addEventListener("click", toggleMute);
document.getElementById("endBtn").addEventListener("click", endCall);
document.getElementById("copyBtn").addEventListener("click", copyLink);
document
	.getElementById("fsLocal")
	.addEventListener("click", () => toggleFullscreen("localWrap"));
document
	.getElementById("fsRemote")
	.addEventListener("click", () => toggleFullscreen("remoteWrap"));
document.getElementById("flipCamera").addEventListener("click", flipCamera);
