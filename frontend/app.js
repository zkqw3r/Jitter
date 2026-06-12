const ROOM_ID = location.pathname.split("/").pop();
const WS_PROTO = location.protocol === "https:" ? "wss:" : "ws:";
const ws = new WebSocket(`${WS_PROTO}//${location.host}/ws/${encodeURIComponent(ROOM_ID)}`);

const $ = (id) => document.getElementById(id);

const STATUS_PRESETS = {
	roomFull: {
		icon: "group",
		title: "Room Full",
		text: "Room Capacity Reached (Max 2).",
	},
	invalidId: {
		icon: "link_off",
		title: "Invalid ID",
		text: "Invalid ID/Link. Check and try again.",
	},
	connectionLost: {
		icon: "wifi_off",
		title: "Connection Lost",
		text: "Lost connection to the server.",
	},
	timeout: {
		icon: "schedule",
		title: "Room Timeout",
		text: "The room has expired due to inactivity.",
	},
	peerLeft: {
		icon: "person_remove",
		title: "Peer Left",
		text: "The other participant has left the room.",
	},
};

function showStatus(presetKey) {
	const preset = STATUS_PRESETS[presetKey];
	if (!preset) return;
	const overlay = $("statusOverlay");
	if (!overlay) return;
	$("statusIcon").textContent = preset.icon;
	$("statusTitle").textContent = preset.title;
	$("statusText").textContent = preset.text;
	overlay.hidden = false;
}


let pc = null;
let iceConfig = { iceServers: [] };
let remoteDescriptionSet = false;
const iceCandidateBuffer = [];
let currentFacingMode = "user";
let localStream = null;

async function loadIceConfig() {
	try {
		const res = await fetch("/api/ice-config", { cache: "no-store" });
		if (!res.ok) throw new Error(`ice-config ${res.status}`);
		return await res.json();
	} catch (err) {
		console.warn("ice-config fallback:", err);
		return { iceServers: [] };
	}
}

function createPeerConnection(config) {
	const peer = new RTCPeerConnection(config);

	peer.onicecandidate = ({ candidate }) => {
		if (candidate && ws.readyState === WebSocket.OPEN) {
			ws.send(JSON.stringify({ type: "ice", candidate }));
		}
	};

	peer.ontrack = ({ streams }) => {
		const video = $("remoteVideo");
		const avatar = $("remoteAvatar");
		if (!video || !streams[0]) return;
		video.srcObject = streams[0];
		streams[0].getVideoTracks().forEach((track) => {
			track.onmute = () => {
				video.style.display = "none";
				if (avatar) {
					avatar.classList.remove("hidden");
					avatar.classList.add("flex");
				}
			};
			track.onunmute = () => {
				video.style.display = "block";
				if (avatar) {
					avatar.classList.add("hidden");
					avatar.classList.remove("flex");
				}
			};
		});
	};

	peer.oniceconnectionstatechange = () => {
		if (peer.iceConnectionState === "failed") {
			console.warn("ICE connection failed; attempting restart");
			try { peer.restartIce(); } catch (_) {}
		}
	};

	return peer;
}


async function acquireMedia() {
	try {
		const stream = await navigator.mediaDevices.getUserMedia({
			video: { facingMode: "user" },
			audio: true,
		});
		return stream;
	} catch (err) {
		console.warn("getUserMedia AV failed, falling back to video-only:", err);
		try {
			const stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: "user" },
				audio: false,
			});
			alert("Микрофон недоступен, работаем только с видео");
			return stream;
		} catch (err2) {
			alert("Не удалось получить доступ к камере/микрофону: " + err2.message);
			return null;
		}
	}
}

async function checkAndShowFlipButton() {
	const isTouch = navigator.maxTouchPoints > 0 || "ontouchstart" in window;
	if (!isTouch) return;
	try {
		const devices = await navigator.mediaDevices.enumerateDevices();
		const videoInputs = devices.filter((d) => d.kind === "videoinput");
		if (videoInputs.length > 1) {
			const flipBtn = $("flipCamera");
			if (flipBtn) flipBtn.classList.remove("hidden");
		}
	} catch (err) {
		console.warn("enumerateDevices failed:", err);
	}
}

async function flipCamera() {
	if (!pc || !localStream) return;
	const videoSender = pc.getSenders().find((s) => s.track?.kind === "video");
	if (!videoSender) return;

	const localVideo = $("localVideo");
	const nextFacingMode = currentFacingMode === "user" ? "environment" : "user";

	const audioTracks = localStream.getAudioTracks();
	localStream.getVideoTracks().forEach((t) => t.stop());
	if (videoSender.track) videoSender.track.stop();

	try {
		const newStream = await navigator.mediaDevices.getUserMedia({
			video: { facingMode: { ideal: nextFacingMode } },
			audio: false,
		});
		const newTrack = newStream.getVideoTracks()[0];
		await videoSender.replaceTrack(newTrack);

		localStream = new MediaStream([newTrack, ...audioTracks]);
		localVideo.srcObject = localStream;

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
			localStream = new MediaStream([fallbackTrack, ...audioTracks]);
			localVideo.srcObject = localStream;
		} catch (fallbackErr) {
			console.error("flipCamera fallback failed:", fallbackErr);
		}
	}
}


async function startCall() {
	if (!pc) return;
	if (pc.signalingState !== "stable") return;
	const offer = await pc.createOffer();
	await pc.setLocalDescription(offer);
	if (ws.readyState === WebSocket.OPEN) {
		ws.send(JSON.stringify({ type: "offer", sdp: offer }));
	}
}

function teardownRemote() {
	const video = $("remoteVideo");
	if (video) {
		try { video.srcObject?.getTracks().forEach((t) => t.stop()); } catch (_) {}
		video.srcObject = null;
		video.style.display = "none";
	}
	const avatar = $("remoteAvatar");
	if (avatar) {
		avatar.classList.remove("hidden");
		avatar.classList.add("flex");
	}
}

async function endCall() {
	try { localStream?.getTracks().forEach((t) => t.stop()); } catch (_) {}
	try { pc?.getSenders().forEach((s) => s.track?.stop()); } catch (_) {}
	try { pc?.close(); } catch (_) {}
	try { ws.close(1000, "bye"); } catch (_) {}
	location.href = "/";
}


ws.onclose = (event) => {
	if (event.code === 4000) {
		showStatus("roomFull");
		return;
	}
	if (event.code !== 1000 && event.code !== 1001) {
		showStatus("connectionLost");
	}
};

ws.onerror = () => {
	showStatus("connectionLost");
};

ws.onmessage = async ({ data }) => {
	let msg;
	try {
		msg = JSON.parse(data);
	} catch (_) {
		return;
	}
	if (!msg || typeof msg !== "object" || typeof msg.type !== "string") return;

	try {
		switch (msg.type) {
			case "offer":
				if (!pc || pc.signalingState !== "stable") return;
				if (!msg.sdp || typeof msg.sdp !== "object") return;
				await pc.setRemoteDescription(msg.sdp);
				remoteDescriptionSet = true;
				await drainIceBuffer();
				await streamReady;
				{
					const answer = await pc.createAnswer();
					await pc.setLocalDescription(answer);
					if (ws.readyState === WebSocket.OPEN) {
						ws.send(JSON.stringify({ type: "answer", sdp: answer }));
					}
				}
				break;

			case "answer":
				if (!pc || pc.signalingState !== "have-local-offer") return;
				if (!msg.sdp || typeof msg.sdp !== "object") return;
				await pc.setRemoteDescription(msg.sdp);
				remoteDescriptionSet = true;
				await drainIceBuffer();
				break;

			case "ice":
				if (!pc) return;
				if (msg.candidate && typeof msg.candidate !== "object") return;
				if (remoteDescriptionSet) {
					try { await pc.addIceCandidate(msg.candidate); } catch (e) {
						console.warn("addIceCandidate failed:", e);
					}
				} else {
					iceCandidateBuffer.push(msg.candidate);
				}
				break;

			case "cam-off":
				setRemoteVideoVisibility(false);
				break;

			case "cam-on":
				setRemoteVideoVisibility(true);
				break;

			case "peer-joined":
				if (pc && pc.signalingState === "stable") {
					await startCall();
				}
				break;

			case "peer-left":
				remoteDescriptionSet = false;
				iceCandidateBuffer.length = 0;
				teardownRemote();
				showStatus("peerLeft");
				break;

			case "room-timeout":
				showStatus("timeout");
				setTimeout(endCall, 1500);
				break;
		}
	} catch (err) {
		console.error("ws.onmessage error:", err);
	}
};

async function drainIceBuffer() {
	for (const candidate of iceCandidateBuffer) {
		try { await pc.addIceCandidate(candidate); } catch (e) {
			console.warn("buffered addIceCandidate failed:", e);
		}
	}
	iceCandidateBuffer.length = 0;
}

function setRemoteVideoVisibility(on) {
	const video = $("remoteVideo");
	const avatar = $("remoteAvatar");
	if (video) video.style.display = on ? "block" : "none";
	if (avatar) {
		avatar.classList.toggle("hidden", on);
		avatar.classList.toggle("flex", !on);
	}
}


function toggleFullscreen(wrapId) {
	const wrap = $(wrapId);
	if (!wrap) return;
	if (!document.fullscreenElement) {
		wrap.requestFullscreen?.();
	} else {
		document.exitFullscreen?.();
	}
}
function toggleMic() {
	if (!localStream) return;
	const tracks = localStream.getAudioTracks();
	if (tracks.length === 0) {
		alert("Микрофон не подключен");
		return;
	}
	const track = tracks[0];
	track.enabled = !track.enabled;
	const isMuted = !track.enabled;

	const btn = $("micBtn");
	const icon = btn.querySelector("span");
	
	if (isMuted) {
		btn.classList.add("off");
		if (icon) icon.textContent = "mic_off";
	} else {
		btn.classList.remove("off");
		if (icon) icon.textContent = "mic";
	}
	btn.title = isMuted ? "Включить микрофон" : "Выключить микрофон";
}

function toggleCam() {
	if (!localStream) return;
	const tracks = localStream.getVideoTracks();
	if (tracks.length === 0) {
		alert("Камера не подключена");
		return;
	}
	const track = tracks[0];
	track.enabled = !track.enabled;
	const isOff = !track.enabled;

	if (ws.readyState === WebSocket.OPEN) {
		ws.send(JSON.stringify({ type: isOff ? "cam-off" : "cam-on" }));
	}

	const btn = $("camBtn");
	const icon = btn.querySelector("span");
	const video = $("localVideo");
	const avatar = $("localAvatar");

	if (isOff) {
		btn.classList.add("off");
		if (icon) icon.textContent = "videocam_off";
		if (video) video.style.display = "none";
		if (avatar) {
			avatar.classList.remove("hidden");
			avatar.classList.add("flex");
		}
	} else {
		btn.classList.remove("off");
		if (icon) icon.textContent = "videocam";
		if (video) video.style.display = "block";
		if (avatar) {
			avatar.classList.add("hidden");
			avatar.classList.remove("flex");
		}
	}
	btn.title = isOff ? "Включить камеру" : "Выключить камеру";
}

function toggleMute() {
	const video = $("remoteVideo");
	if (!video) return;
	video.muted = !video.muted;
	const isMuted = video.muted;

	const btn = $("muteBtn");
	const icon = btn.querySelector("span");
	
	if (isMuted) {
		btn.classList.add("off");
		if (icon) icon.textContent = "volume_off";
	} else {
		btn.classList.remove("off");
		if (icon) icon.textContent = "volume_up";
	}
	btn.title = isMuted ? "Включить звук" : "Выключить звук";
}

function copyLink() {
	navigator.clipboard.writeText(location.href).then(() => {
		const btn = $("copyBtn");
		const icon = btn?.querySelector("span");
		if (!icon) return;
		icon.textContent = "check";
		btn.title = "Скопировано!";
		setTimeout(() => {
			icon.textContent = "link";
			btn.title = "Скопировать ссылку";
		}, 2000);
	});
}


const streamReady = (async () => {
	iceConfig = await loadIceConfig();
	pc = createPeerConnection(iceConfig);

	localStream = await acquireMedia();
	if (localStream) {
		$("localVideo").srcObject = localStream;
		localStream.getTracks().forEach((t) => pc.addTrack(t, localStream));
		checkAndShowFlipButton();
	}
})();

$("micBtn")?.addEventListener("click", toggleMic);
$("camBtn")?.addEventListener("click", toggleCam);
$("muteBtn")?.addEventListener("click", toggleMute);
$("endBtn")?.addEventListener("click", endCall);
$("copyBtn")?.addEventListener("click", copyLink);
$("fsLocal")?.addEventListener("click", () => toggleFullscreen("localWrap"));
$("fsRemote")?.addEventListener("click", () => toggleFullscreen("remoteWrap"));
$("flipCamera")?.addEventListener("click", flipCamera);
