const endpoints = {
  status: "/yinqiao/status",
  network: "/yinqiao/network",
  enroll: "/yinqiao/enroll",
	accounts: "/yinqiao/accounts",
	settings: "/yinqiao/settings",
  capabilities: "/yinqiao/capabilities",
  apiCapabilities: "/yinqiao/api-capabilities",
};

const elements = {
  healthBadge: document.querySelector("#healthBadge"),
  healthText: document.querySelector("#healthText"),
  apiState: document.querySelector("#apiState"),
  bindingState: document.querySelector("#bindingState"),
  versionLabel: document.querySelector("#versionLabel"),
  accountList: document.querySelector("#accountList"),
	accountEmpty: document.querySelector("#accountEmpty"),
	form: document.querySelector("#enrollmentForm"),
  username: document.querySelector("#username"),
  password: document.querySelector("#password"),
  passwordToggle: document.querySelector("#passwordToggle"),
  submitButton: document.querySelector("#submitButton"),
  formMessage: document.querySelector("#formMessage"),
  refreshButton: document.querySelector("#refreshButton"),
  openMusicLink: document.querySelector("#openMusicLink"),
	addAccountButton: document.querySelector("#addAccountButton"),
	cancelAccountButton: document.querySelector("#cancelAccountButton"),
	settingsForm: document.querySelector("#settingsForm"),
	protocolPort: document.querySelector("#protocolPort"),
	advertisedHost: document.querySelector("#advertisedHost"),
	toggleSubsonic: document.querySelector("#toggleSubsonic"),
	toggleSynology: document.querySelector("#toggleSynology"),
	toggleJellyfin: document.querySelector("#toggleJellyfin"),
	toggleAmpache: document.querySelector("#toggleAmpache"),
	settingsState: document.querySelector("#settingsState"),
	settingsAddressPreview: document.querySelector("#settingsAddressPreview"),
	sidebarPort: document.querySelector("#sidebarPort"),
	saveSettingsButton: document.querySelector("#saveSettingsButton"),
	resetSettingsButton: document.querySelector("#resetSettingsButton"),
  apiVersion: document.querySelector("#apiVersion"),
  apiCounts: document.querySelector("#apiCounts"),
  apiStats: document.querySelector("#apiStats"),
  apiLayers: document.querySelector("#apiLayers"),
  toast: document.querySelector("#toast"),
};

const apiCategoryCopy = {
  system: ["服务发现", "读取配置、版本和初始化状态。"],
  auth: ["登录鉴权", "建立、检查和结束飞牛音乐登录态。"],
  users: ["用户与权限", "管理音乐账号、密码、权限和封禁状态。"],
  settings: ["服务设置", "读取或修改音乐服务器及默认授权设置。"],
  directories: ["授权目录", "浏览 fnOS 已授权给音乐应用的存储目录。"],
  libraries: ["共享曲库", "创建、维护并扫描飞牛音乐共享曲库。"],
  catalog: ["曲库浏览", "读取歌曲、专辑、歌手、风格与歌词数据。"],
  playback: ["播放与转码", "提供原音频、HLS、漫游播放和转码会话。"],
  metadata: ["元数据维护", "创建分类资料并编辑歌曲元数据和封面。"],
  media: ["封面与媒体", "读取或上传歌曲、歌单的媒体资源。"],
  activity: ["播放事件", "向飞牛音乐上报播放进度和用户偏好事件。"],
  personal: ["收藏与历史", "读取和维护用户收藏、播放历史及失效记录。"],
  playlists: ["播放列表", "创建歌单并维护歌单详情、曲目和封面。"],
  search: ["搜索服务", "搜索各类音乐内容、联想词并维护搜索索引。"],
  tasks: ["后台任务", "查看、取消、删除或重试曲库后台任务。"],
};

const apiLayerCopy = [
  { title: "接入与身份", description: "客户端建立连接、完成初始化并取得飞牛音乐身份。", categories: ["system", "auth"] },
  { title: "服务器与曲库管理", description: "管理员维护用户、目录、共享曲库、元数据和后台任务。", categories: ["users", "settings", "directories", "libraries", "metadata", "tasks"] },
  { title: "曲库浏览与检索", description: "客户端获取完整音乐目录，并按不同维度查询内容。", categories: ["catalog", "search"] },
  { title: "播放与媒体", description: "输出封面和音频，支持断点读取、HLS 与转码生命周期。", categories: ["playback", "media"] },
  { title: "个人音乐数据", description: "同步收藏、历史、歌单和播放行为。", categories: ["personal", "playlists", "activity"] },
];

const apiAccessCopy = {
  public: ["公开", "无需音乐登录态"],
  user: ["用户", "需要飞牛音乐用户登录态"],
  admin: ["管理员", "需要飞牛音乐管理员权限"],
  media: ["媒体", "用于封面与音频资源读取"],
};

const apiResponseCopy = { json: "JSON", raw: "二进制", upload: "文件上传" };

const thirdPartyApps = [
  { name: "箭头", fullName: "箭头音乐", note: "iPhone 音乐客户端", icon: "assets/apps/arrow.jpg", source: "https://www.amcfy.com/docs", platforms: ["iphone"], protocols: ["subsonic", "synology", "jellyfin"] },
  { name: "音流", fullName: "音流 StreamMusic", note: "多平台，优先推荐", icon: "assets/apps/streammusic.jpg", source: "https://github.com/gitbobobo/StreamMusic", platforms: ["android", "iphone", "windows", "mac"], protocols: ["subsonic", "synology", "jellyfin"], recommended: true },
  { name: "Symfonium", note: "Android 专业播放器", icon: "assets/apps/symfonium.png", source: "https://symfonium.app/", platforms: ["android"], protocols: ["subsonic", "jellyfin"] },
  { name: "Amperfy", note: "Apple 生态播放器", icon: "assets/apps/amperfy.jpg", source: "https://github.com/BLeeEZ/amperfy", platforms: ["iphone", "mac"], protocols: ["subsonic", "ampache"] },
  { name: "Substreamer", note: "移动端 OpenSubsonic", icon: "assets/apps/substreamer.jpg", source: "https://substreamer.org/", platforms: ["android", "iphone"], protocols: ["subsonic"] },
  { name: "play:Sub", note: "iOS Subsonic 客户端", icon: "assets/apps/playsub.jpg", source: "https://michaelsapps.dk/playsubapp/", platforms: ["iphone"], protocols: ["subsonic"] },
  { name: "Ultrasonic", note: "Android 开源客户端", icon: "assets/apps/ultrasonic.png", source: "https://ultrasonic.gitlab.io/", platforms: ["android"], protocols: ["subsonic"] },
  { name: "Feishin", note: "现代桌面音乐播放器", icon: "assets/apps/feishin.png", source: "https://github.com/jeffvli/feishin", platforms: ["windows", "mac"], protocols: ["subsonic", "jellyfin"] },
  { name: "Supersonic", note: "轻量桌面播放器", icon: "assets/apps/supersonic.png", source: "https://github.com/dweymouth/supersonic", platforms: ["windows", "mac"], protocols: ["subsonic"] },
  { name: "Finamp", note: "Jellyfin 移动音乐客户端", icon: "assets/apps/finamp.jpg", source: "https://github.com/jmshrv/finamp", platforms: ["android", "iphone"], protocols: ["jellyfin"] },
  { name: "Fintunes", note: "Jellyfin 移动音频客户端", icon: "assets/apps/fintunes.png", source: "https://www.fintunes.app/", platforms: ["android", "iphone"], protocols: ["jellyfin"] },
  { name: "Jellyfin", fullName: "Jellyfin Media Player", note: "Jellyfin 官方桌面客户端", icon: "assets/apps/jellyfin.png", source: "https://jellyfin.org/downloads/clients/all/", platforms: ["windows", "mac"], protocols: ["jellyfin"] },
  { name: "DS audio", note: "Synology 官方移动客户端", icon: "assets/apps/dsaudio.jpg", source: "https://www.synology.com/dsm/feature/audio_station", platforms: ["android", "iphone"], protocols: ["synology"] },
  { name: "Power Ampache", fullName: "Power Ampache 2", note: "Android Ampache 客户端", icon: "assets/apps/powerampache.png", source: "https://power.ampache.dev/", platforms: ["android"], protocols: ["ampache"] },
  { name: "Amarok", note: "桌面 Ampache 客户端", icon: "assets/apps/amarok.svg", source: "https://ampache.org/docs/clients/api/", platforms: ["windows", "mac"], protocols: ["ampache"] },
];

const staticAPIRoutes = Array.isArray(window.API_CAPABILITY_MANIFEST) ? window.API_CAPABILITY_MANIFEST : [];

const capabilityCopy = {
  "authentication": ["账号登录", "可使用已绑定的飞牛音乐账号登录客户端。"],
  "password authentication": ["密码登录", "可直接使用飞牛音乐账号密码完成登录。"],
  "digest handshake": ["安全握手", "可完成 Ampache 客户端要求的摘要认证。"],
  "public server discovery": ["服务器发现", "客户端可以识别这是一个可连接的 Jellyfin 服务。"],
  "users": ["用户信息", "客户端可以读取当前登录用户和基础权限。"],
  "artists": ["歌手浏览", "可以按歌手查看飞牛音乐曲库。"],
  "Artist": ["歌手浏览", "可以读取歌手列表和歌手详情。"],
  "albums": ["专辑浏览", "可以浏览专辑并进入专辑歌曲列表。"],
  "Album": ["专辑浏览", "可以读取专辑列表和专辑内容。"],
  "songs": ["歌曲浏览", "可以读取歌曲列表、详情和播放信息。"],
  "Song": ["歌曲浏览", "可以读取歌曲列表、详情和播放信息。"],
  "genres": ["流派分类", "可以按音乐流派筛选曲库。"],
  "Genre": ["流派分类", "可以浏览飞牛音乐中的流派分类。"],
  "Folder": ["目录浏览", "可以按飞牛音乐曲库目录逐层浏览歌曲。"],
  "music views": ["音乐资料库", "客户端会显示独立的飞牛音乐资料库入口。"],
  "search": ["曲库搜索", "可以搜索歌曲、专辑和歌手。"],
  "Search": ["曲库搜索", "可以在 Audio Station 客户端中搜索曲库。"],
  "favorites": ["歌曲收藏", "可以查看和修改飞牛音乐收藏状态。"],
  "favorites for songs": ["歌曲收藏", "可以收藏或取消收藏单曲。"],
  "song rating as favorite": ["歌曲收藏", "Audio Station 的评分操作会映射为收藏或取消收藏。"],
  "playlists": ["播放列表", "可以查看、创建和维护播放列表。"],
  "Playlist": ["播放列表", "可以查看并修改飞牛音乐播放列表。"],
  "cover art": ["封面图片", "可以显示歌曲、专辑和歌手封面。"],
  "Cover": ["封面图片", "可以读取飞牛音乐中的封面图片。"],
  "images": ["媒体图片", "可以读取 Jellyfin 客户端需要的封面资源。"],
  "Range streaming": ["断点流式播放", "支持分段读取音频，拖动进度和后台播放可正常工作。"],
  "direct Range streaming": ["原音质直连播放", "音频由飞牛音乐直接输出，并支持拖动播放进度。"],
  "Stream": ["在线播放", "可以直接播放飞牛音乐中的原始音频。"],
  "download": ["歌曲下载", "客户端可以下载原始音频用于离线播放。"],
  "Download": ["歌曲下载", "可以从飞牛音乐下载原始歌曲文件。"],
  "lyrics": ["歌词", "可以读取飞牛音乐中的歌曲歌词。"],
  "Lyrics": ["歌词", "可以在客户端中显示飞牛音乐歌词。"],
  "play reporting": ["播放记录", "客户端可上报开始、进度和停止播放事件。"],
  "scan status": ["扫描状态", "客户端可以查看飞牛音乐曲库扫描状态。"],
  "SYNO.API.Info": ["接口发现", "Audio Station 客户端可以查询当前提供的接口版本。"],
  "SYNO.API.Auth": ["账号认证", "支持 Audio Station 登录、会话保持和退出。"],
  "SYNO.AudioStation.Info": ["服务信息", "客户端可以确认 Audio Station 音乐服务可用。"],
  "podcasts": ["播客", "不提供播客订阅、节目列表或播客播放。"],
  "internet radio": ["网络电台", "不提供网络电台收藏和在线播放。"],
  "radio": ["网络电台", "不提供 Ampache 网络电台能力。"],
  "Radio": ["网络电台", "不提供 Audio Station 网络电台能力。"],
  "video": ["视频播放", "仅桥接音乐曲库，不提供视频媒体库。"],
  "Live TV": ["电视直播", "不提供 Jellyfin 电视直播和节目单。"],
  "chat": ["聊天", "不提供 Subsonic 用户聊天功能。"],
  "shares": ["公开分享", "不能生成对外公开访问的歌曲分享链接。"],
  "jukebox": ["服务器声卡播放", "不能控制 NAS 声卡在服务器端直接播放。"],
  "bookmarks": ["有声书书签", "不保存有声书或长音频的播放书签。"],
  "server-side play queue persistence": ["云端播放队列", "播放队列不会保存在服务器并跨客户端同步。"],
  "catalog administration": ["曲库管理", "客户端不能通过 Ampache 接口新增或重新扫描曲库。"],
  "user administration": ["用户管理", "客户端不能新增、删除或修改飞牛音乐用户。"],
  "local playback control": ["本机播放控制", "不能遥控 NAS 本机的播放器或音量。"],
  "QuickConnect discovery": ["QuickConnect", "不支持群晖 QuickConnect 地址发现，请直接填写服务器地址。"],
  "OTP": ["双重验证码", "Audio Station 兼容登录不支持群晖 OTP 验证流程。"],
  "DSM administration": ["DSM 系统管理", "不能通过本服务修改 NAS 系统设置。"],
  "RemotePlayer": ["远程播放器", "不能遥控另一台 Audio Station 播放设备。"],
  "MediaServer": ["DLNA 媒体服务器", "不模拟群晖 Media Server 或 DLNA 服务。"],
  "LyricsSearch plugins": ["在线搜歌词", "不调用群晖歌词插件，只读取飞牛音乐已有歌词。"],
  "smart playlists": ["智能播放列表", "不能创建按规则自动更新的智能歌单。"],
  "playlist sharing": ["歌单公开分享", "不能生成公开访问的播放列表分享链接。"],
  "WebPlayer queue": ["网页播放队列", "不保存群晖网页播放器的云端队列。"],
  "composer index": ["作曲家分类", "暂不按作曲家建立独立浏览索引。"],
  "channels": ["在线频道", "不提供 Jellyfin 插件频道和在线内容。"],
  "plugins": ["服务端插件", "不支持安装或调用 Jellyfin 插件。"],
  "remote session control": ["远程播放控制", "不能控制其他 Jellyfin 客户端的播放会话。"],
  "SyncPlay": ["同步播放", "不支持多台设备同步播放同一首歌曲。"],
  "WebSocket events": ["实时事件推送", "不会通过 WebSocket 推送曲库或会话变化。"],
  "subtitle APIs": ["字幕", "音乐服务不提供视频字幕接口。"],
  "hardware transcoding": ["硬件转码", "不进行音视频硬件转码，歌曲以原文件直连。"],
  "Jellyfin administration": ["Jellyfin 管理后台", "不能修改 Jellyfin 用户、插件或服务器设置。"],
  "album and artist favorites": ["专辑和歌手收藏", "目前只同步单曲收藏，不同步专辑或歌手收藏。"],
};

let accountFormOpen = false;
let currentBoundUsers = [];
let detectedNetworkAddress = "";
let detectedNetworkHost = "";
let toastTimer;
let viewAnimation;
let illuminatedSurface;
let illuminationFrame;

const playerAddressElements = document.querySelectorAll("#serverAddress, #subsonicAddress, #ampacheAddress, #synologyAddress, #jellyfinAddress");

function setPlayerAddress(address, detected = false) {
  playerAddressElements.forEach((element) => { element.textContent = address; });
	if (detected) {
		detectedNetworkAddress = address;
		try {
			detectedNetworkHost = new URL(address).hostname;
		} catch (_) {
			detectedNetworkHost = "";
		}
		elements.advertisedHost.placeholder = detectedNetworkHost ? `自动：${detectedNetworkHost}` : "自动检测";
		elements.advertisedHost.title = detectedNetworkHost ? `当前自动检测地址：${detectedNetworkHost}` : "自动检测 NAS 局域网地址";
	}
	updateSettingsPreview();
}

async function loadNetworkAddress() {
  const response = await fetch(endpoints.network, { headers: { Accept: "application/json" }, cache: "no-store" });
  const payload = await readJSON(response);
  if (!response.ok || !payload.ok || !payload.address) throw new Error("未检测到 NAS 局域网地址");
	setPlayerAddress(payload.address, true);
}

setPlayerAddress("http://NAS局域网IP:14040");

function configureMusicLink() {
  try {
    if (document.referrer) {
      elements.openMusicLink.href = new URL("/music/settings?key=user-management", document.referrer).href;
      return;
    }
  } catch (_) {
    // Some fnOS configurations hide the desktop referrer.
  }
  elements.openMusicLink.href = "/music/settings?key=user-management";
}

function activateView(name, updateHash = true) {
  const nextSection = document.querySelector(`[data-view="${name}"]`);
  const nextTab = document.querySelector(`[data-view-target="${name}"]`);
  if (!nextSection || !nextTab) return;

  document.querySelectorAll("[data-view]").forEach((section) => {
    const active = section === nextSection;
    section.hidden = !active;
    section.classList.toggle("is-active", active);
  });
  document.querySelectorAll("[data-view-target]").forEach((tab) => {
    const active = tab === nextTab;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
    tab.tabIndex = active ? 0 : -1;
  });

  if (viewAnimation) viewAnimation.cancel();
  if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches && nextSection.animate) {
    viewAnimation = nextSection.animate(
      [{ opacity: 0.35, transform: "translateY(6px)" }, { opacity: 1, transform: "translateY(0)" }],
      { duration: 210, easing: "cubic-bezier(.2,.8,.2,1)" },
    );
  }
  if (updateHash && window.location.hash !== `#${name}`) history.replaceState(null, "", `#${name}`);
  document.querySelector(".content-area").scrollTo({ top: 0, behavior: "auto" });
}

document.querySelectorAll("[data-view-target]").forEach((tab) => {
  tab.addEventListener("click", () => activateView(tab.dataset.viewTarget));
  tab.addEventListener("keydown", (event) => {
    if (!["ArrowDown", "ArrowUp", "ArrowLeft", "ArrowRight"].includes(event.key)) return;
    event.preventDefault();
    const tabs = [...document.querySelectorAll("[data-view-target]")];
    const direction = event.key === "ArrowDown" || event.key === "ArrowRight" ? 1 : -1;
    const next = tabs[(tabs.indexOf(tab) + direction + tabs.length) % tabs.length];
    next.focus();
    activateView(next.dataset.viewTarget);
  });
});

function setServiceState(element, enabled, enabledLabel = "已开启") {
  element.textContent = enabled ? enabledLabel : "未开启";
  element.classList.toggle("enabled", enabled);
  element.classList.toggle("error", !enabled);
}

function setProtocolState(name, enabled) {
	document.querySelectorAll(`[data-protocol-state="${name}"]`).forEach((element) => {
		element.textContent = enabled ? "已开启" : "已关闭";
		element.classList.toggle("enabled", enabled);
		element.classList.toggle("disabled", !enabled);
		element.classList.remove("error");
	});
}

function setAPIState(enabled, enabledLabel = "已连接") {
  document.querySelectorAll("#apiState, [data-api-state]").forEach((element) => setServiceState(element, enabled, enabledLabel));
}

function renderAccounts(users = []) {
  currentBoundUsers = users;
  elements.accountList.replaceChildren();
  const hasUsers = users.length > 0;
	elements.accountEmpty.hidden = hasUsers;
	if (!hasUsers) accountFormOpen = true;
	elements.form.hidden = !accountFormOpen;
	elements.cancelAccountButton.hidden = !hasUsers;
	elements.bindingState.textContent = hasUsers ? `${users.length} 个账号` : "未绑定";
  elements.bindingState.classList.toggle("bound", hasUsers);
  for (const username of users) {
		const row = document.createElement("div");
		row.className = "account-row";
		const avatar = document.createElement("span");
		avatar.className = "account-avatar";
		avatar.textContent = username.slice(0, 1).toUpperCase();
		const copy = document.createElement("div");
		const name = document.createElement("strong");
		name.textContent = username;
		const note = document.createElement("span");
		note.textContent = "飞牛音乐账号 · 已加密保存";
		copy.append(name, note);
		const remove = document.createElement("button");
		remove.className = "icon-button account-delete";
		remove.type = "button";
		remove.title = `删除账号 ${username}`;
		remove.setAttribute("aria-label", remove.title);
		remove.innerHTML = '<svg aria-hidden="true"><use href="#icon-trash"></use></svg>';
		remove.addEventListener("click", () => deleteAccount(username, remove));
		row.append(avatar, copy, remove);
		elements.accountList.append(row);
  }
}

async function loadAccounts() {
	const response = await fetch(endpoints.accounts, { headers: { Accept: "application/json" }, cache: "no-store" });
	const payload = await readJSON(response);
	if (!response.ok || !payload.ok) throw new Error(payload.error || "无法读取绑定账号");
	renderAccounts(Array.isArray(payload.accounts) ? payload.accounts : []);
}

async function deleteAccount(username, button) {
	button.disabled = true;
	try {
		const response = await fetch(`${endpoints.accounts}?username=${encodeURIComponent(username)}`, { method: "DELETE", headers: { Accept: "application/json" } });
		const payload = await readJSON(response);
		if (!response.ok || !payload.ok) throw new Error(payload.error || "删除失败");
		showToast(`账号 ${username} 已删除`);
		renderAccounts(Array.isArray(payload.accounts) ? payload.accounts : []);
	} catch (error) {
		showToast(error.message || "删除账号失败", "error");
	} finally {
		button.disabled = false;
	}
}

async function readJSON(response) {
  try {
    return await response.json();
  } catch (_) {
    throw new Error(`服务返回了无法识别的响应（HTTP ${response.status}）`);
  }
}

async function loadStatus() {
  elements.healthBadge.dataset.state = "loading";
  elements.healthText.textContent = "正在检查";
  document.querySelectorAll("#apiState, [data-api-state]").forEach((element) => {
    element.textContent = "检查中";
    element.classList.remove("enabled", "error");
  });
  elements.refreshButton.disabled = true;
  elements.refreshButton.classList.add("is-spinning");
  try {
    const response = await fetch(endpoints.status, { headers: { Accept: "application/json" }, cache: "no-store" });
    const payload = await readJSON(response);
    if (!response.ok || !payload.ok) throw new Error(payload.error || "服务状态不可用");
    elements.healthBadge.dataset.state = "ok";
    elements.healthText.textContent = "运行正常";
    setAPIState(true);
    elements.versionLabel.textContent = payload.version ? `v${payload.version}` : "版本未知";
    if (payload.protocols) {
      for (const name of ["openSubsonic", "ampache", "synology", "jellyfin"]) setProtocolState(name, Boolean(payload.protocols[name]?.enabled));
    }
  } catch (error) {
    elements.healthBadge.dataset.state = "error";
    elements.healthText.textContent = "需要检查";
    setAPIState(false);
    document.querySelectorAll("#apiState, [data-api-state]").forEach((element) => { element.textContent = "连接失败"; });
    const message = /cannot reach|API/i.test(error.message || "")
      ? "无法连接本机 Yinqiao API，请检查飞牛音乐和音桥服务"
      : (error.message || "无法读取服务状态");
    showToast(message, "error");
  } finally {
    elements.refreshButton.disabled = false;
    elements.refreshButton.classList.remove("is-spinning");
  }
}

function settingsPayloadFromForm() {
	return {
		protocolPort: Number(elements.protocolPort.value),
		advertisedHost: elements.advertisedHost.value.trim(),
		protocols: {
			openSubsonic: elements.toggleSubsonic.checked,
			synology: elements.toggleSynology.checked,
			jellyfin: elements.toggleJellyfin.checked,
			ampache: elements.toggleAmpache.checked,
		},
	};
}

function renderSettings(settings) {
	elements.protocolPort.value = settings.protocolPort;
	elements.advertisedHost.value = settings.advertisedHost || "";
	elements.toggleSubsonic.checked = Boolean(settings.protocols?.openSubsonic);
	elements.toggleSynology.checked = Boolean(settings.protocols?.synology);
	elements.toggleJellyfin.checked = Boolean(settings.protocols?.jellyfin);
	elements.toggleAmpache.checked = Boolean(settings.protocols?.ampache);
	elements.sidebarPort.textContent = settings.protocolPort;
	document.querySelectorAll("[data-protocol-port]").forEach((element) => { element.textContent = settings.protocolPort; });
	setProtocolState("openSubsonic", elements.toggleSubsonic.checked);
	setProtocolState("synology", elements.toggleSynology.checked);
	setProtocolState("jellyfin", elements.toggleJellyfin.checked);
	setProtocolState("ampache", elements.toggleAmpache.checked);
	elements.settingsState.textContent = "已同步";
	elements.settingsState.classList.remove("dirty", "error");
	updateSettingsPreview();
}

function updateSettingsPreview() {
	const host = elements.advertisedHost.value.trim();
	const port = elements.protocolPort.value || "14040";
	if (!host && detectedNetworkAddress) {
		try {
			const detected = new URL(detectedNetworkAddress);
			detected.port = port;
			elements.settingsAddressPreview.textContent = detected.origin;
			return;
		} catch (_) {
			// Fall back to the readable placeholder below.
		}
	}
	const effectiveHost = host || detectedNetworkHost || "NAS局域网IP";
	const displayHost = effectiveHost.includes(":") && !effectiveHost.startsWith("[") ? `[${effectiveHost}]` : effectiveHost;
	elements.settingsAddressPreview.textContent = `http://${displayHost}:${port}`;
}

function markSettingsDirty() {
	elements.settingsState.textContent = "待保存";
	elements.settingsState.classList.add("dirty");
	elements.settingsState.classList.remove("error");
	updateSettingsPreview();
}

async function loadSettings() {
	const response = await fetch(endpoints.settings, { headers: { Accept: "application/json" }, cache: "no-store" });
	const payload = await readJSON(response);
	if (!response.ok || !payload.ok) throw new Error(payload.error || "无法读取设置");
	renderSettings(payload.settings);
}

function protocolKeyFromName(name = "") {
  if (name.includes("OpenSubsonic")) return "openSubsonic";
  if (name.includes("Synology")) return "synology";
  if (name.includes("Jellyfin")) return "jellyfin";
  if (name.includes("Ampache")) return "ampache";
  return "";
}

function capabilityItem(value, supported) {
  const copy = capabilityCopy[value] || [value, supported ? "该接口已桥接到飞牛音乐，可以在客户端中使用。" : "飞牛音乐当前没有可对应的能力，因此不提供此接口。"];
  const item = document.createElement("div");
  item.className = `capability-item ${supported ? "supported" : "unsupported"}`;
  const indicator = document.createElement("span");
  indicator.className = "capability-indicator";
  indicator.textContent = supported ? "✓" : "×";
  indicator.setAttribute("aria-hidden", "true");
  const content = document.createElement("div");
  const heading = document.createElement("div");
  heading.className = "capability-item-heading";
  const title = document.createElement("strong");
  title.textContent = copy[0];
  const technicalName = document.createElement("code");
  technicalName.textContent = value;
  heading.append(title, technicalName);
  const description = document.createElement("p");
  description.textContent = copy[1];
  content.append(heading, description);
  item.append(indicator, content);
  return item;
}

function capabilityPanel(title, values, supported) {
  const panel = document.createElement("section");
  panel.className = `capability-panel glass-surface ${supported ? "supported" : "unsupported"}`;
  const header = document.createElement("header");
  const label = document.createElement("h3");
  label.textContent = title;
  const count = document.createElement("span");
  count.textContent = `${values.length} 项`;
  header.append(label, count);
  const items = document.createElement("div");
  items.className = "capability-items";
  for (const value of values) items.append(capabilityItem(value, supported));
  panel.append(header, items);
  return panel;
}

function renderProtocolCapability(protocol) {
  const key = protocolKeyFromName(protocol.name);
  const root = document.querySelector(`[data-capability-root="${key}"]`);
  if (!root) return;
  const supported = Array.isArray(protocol.supported) ? protocol.supported : [];
  const unsupported = Array.isArray(protocol.unsupported) ? protocol.unsupported : [];
  const counts = root.querySelector(".capability-counts");
  counts.replaceChildren();
  const supportedCount = document.createElement("span");
  supportedCount.className = "count-chip";
  supportedCount.textContent = `${supported.length} 项支持`;
  const unsupportedCount = document.createElement("span");
  unsupportedCount.className = "count-chip unsupported";
  unsupportedCount.textContent = `${unsupported.length} 项不支持`;
  counts.append(supportedCount, unsupportedCount);
  const columns = root.querySelector(".capability-columns");
  columns.replaceChildren(capabilityPanel("支持的音乐能力", supported, true), capabilityPanel("当前不支持", unsupported, false));
  setProtocolState(key, Boolean(protocol.enabled));
}

async function loadCapabilities() {
  try {
    const response = await fetch(endpoints.capabilities, { headers: { Accept: "application/json" }, cache: "no-store" });
    const payload = await readJSON(response);
    if (!response.ok || !payload.ok || !Array.isArray(payload.protocols)) throw new Error(payload.error || "无法读取能力声明");
    for (const protocol of payload.protocols) renderProtocolCapability(protocol);
  } catch (error) {
    document.querySelectorAll(".capability-columns").forEach((columns) => {
      columns.replaceChildren();
      const message = document.createElement("div");
      message.className = "loading-surface glass-surface error-text";
      message.textContent = error.message || "无法读取能力声明";
      columns.append(message);
    });
  }
}

function apiRouteItem(route) {
  const item = document.createElement("article");
  item.className = "api-route";
  item.dataset.routeId = route.id || "";

  const heading = document.createElement("div");
  heading.className = "api-route-heading";
  const method = document.createElement("span");
  method.className = `method-chip method-${String(route.method || "get").toLowerCase()}`;
  method.textContent = route.method || "GET";
  const summary = document.createElement("strong");
  summary.textContent = route.summary || route.id || "API 接口";
  const state = document.createElement("span");
  state.className = "route-state is-checking";
  state.textContent = "检测中";
  heading.append(method, summary, state);

  const path = document.createElement("code");
  path.className = "api-route-path";
  path.textContent = route.path || "";

  const meta = document.createElement("div");
  meta.className = "api-route-meta";
  const accessCopy = apiAccessCopy[route.access] || [route.access || "未知权限", "按 API 权限规则访问"];
  const access = document.createElement("span");
  access.className = `access-chip access-${route.access || "unknown"}`;
  access.textContent = accessCopy[0];
  access.title = accessCopy[1];
  const response = document.createElement("span");
  response.textContent = apiResponseCopy[route.responseMode] || route.responseMode || "响应";
  const id = document.createElement("span");
  id.textContent = route.id || "";
  meta.append(access, response, id);

  item.append(heading, path, meta);
  return item;
}

function apiCategory(category, routes, initiallyOpen) {
  const copy = apiCategoryCopy[category] || [category, "由 Yinqiao API 提供的稳定接口能力。"];
  const panel = document.createElement("details");
  panel.className = "api-category glass-surface";
  panel.open = initiallyOpen;

  const summary = document.createElement("summary");
  const mark = document.createElement("span");
  mark.className = "api-category-mark";
  mark.textContent = copy[0].slice(0, 1);
  const heading = document.createElement("span");
  heading.className = "api-category-heading";
  const title = document.createElement("strong");
  title.textContent = copy[0];
  const description = document.createElement("span");
  description.textContent = copy[1];
  heading.append(title, description);
  const count = document.createElement("span");
  count.className = "api-category-count";
  count.textContent = `${routes.length} 个接口`;
  const arrow = document.createElement("span");
  arrow.className = "api-category-arrow";
  arrow.textContent = "›";
  arrow.setAttribute("aria-hidden", "true");
  summary.append(mark, heading, count, arrow);

  const items = document.createElement("div");
  items.className = "api-route-list";
  for (const route of routes) items.append(apiRouteItem(route));
  panel.append(summary, items);
  return panel;
}

function apiStat(value, label, detail) {
  const card = document.createElement("article");
  card.className = "api-stat glass-surface";
  const strong = document.createElement("strong");
  strong.textContent = value;
  const copy = document.createElement("div");
  const title = document.createElement("span");
  title.textContent = label;
  const description = document.createElement("small");
  description.textContent = detail;
  copy.append(title, description);
  card.append(strong, copy);
  return card;
}

function apiPassthrough(passthrough) {
  if (!passthrough?.path) return null;
  const panel = document.createElement("section");
  panel.className = "api-passthrough glass-surface";
  const mark = document.createElement("span");
  mark.className = "api-category-mark passthrough-mark";
  mark.textContent = "C";
  const copy = document.createElement("div");
  const title = document.createElement("strong");
  title.textContent = "官方接口兼容通道";
  const description = document.createElement("p");
  description.textContent = "用于调用尚未纳入稳定层的飞牛音乐官方接口；它保留原始返回结构，底层升级时可能随之变化。";
  const path = document.createElement("code");
  path.textContent = passthrough.path;
  copy.append(title, description, path);
  const methods = document.createElement("span");
  methods.className = "passthrough-methods";
  methods.textContent = Array.isArray(passthrough.methods) ? passthrough.methods.join(" · ") : "GET · POST";
  panel.append(mark, copy, methods);
  return panel;
}

function renderAPIManifest(routes) {
  if (!routes.length) throw new Error("Yinqiao API 静态接口清单缺失");
  const categories = new Map();
  for (const route of routes) {
    const category = route.category || "other";
    if (!categories.has(category)) categories.set(category, []);
    categories.get(category).push(route);
  }

  elements.apiCounts.replaceChildren();
  const supported = document.createElement("span");
  supported.className = "count-chip";
  supported.textContent = `${routes.length} 项接口清单`;
  const detection = document.createElement("span");
  detection.className = "count-chip api-count-secondary is-checking";
  detection.id = "apiDetectionCount";
  detection.textContent = "异步检测中";
  elements.apiCounts.append(supported, detection);

  elements.apiStats.replaceChildren(
    apiStat(String(routes.length), "稳定接口", "底层升级由 API 统一兼容"),
    apiStat(String(categories.size), "能力分类", "按音乐业务层级组织"),
    apiStat(String(new Set(routes.map((route) => route.access)).size), "权限级别", "公开、用户、管理员与媒体"),
    apiStat(String(new Set(routes.map((route) => route.responseMode)).size), "响应形态", "JSON、二进制与文件上传"),
  );

  elements.apiLayers.replaceChildren();
  apiLayerCopy.forEach((layer, layerIndex) => {
    const layerRoutes = layer.categories.flatMap((category) => categories.get(category) || []);
    if (!layerRoutes.length) return;
    const section = document.createElement("section");
    section.className = "api-layer";
    const header = document.createElement("header");
    const heading = document.createElement("div");
    const eyebrow = document.createElement("span");
    eyebrow.textContent = `第 ${layerIndex + 1} 层`;
    const title = document.createElement("h3");
    title.textContent = layer.title;
    const description = document.createElement("p");
    description.textContent = layer.description;
    heading.append(eyebrow, title, description);
    const count = document.createElement("span");
    count.className = "layer-count";
    count.textContent = `${layerRoutes.length} 个接口`;
    header.append(heading, count);
    const grid = document.createElement("div");
    grid.className = "api-category-grid";
    layer.categories.forEach((category, categoryIndex) => {
      const categoryRoutes = categories.get(category);
      if (categoryRoutes?.length) grid.append(apiCategory(category, categoryRoutes, layerIndex === 0 && categoryIndex === 0));
    });
    section.append(header, grid);
    elements.apiLayers.append(section);
  });

  const knownCategories = new Set(apiLayerCopy.flatMap((layer) => layer.categories));
  const uncategorized = [...categories.keys()].filter((category) => !knownCategories.has(category));
  if (uncategorized.length) {
    const section = document.createElement("section");
    section.className = "api-layer";
    const grid = document.createElement("div");
    grid.className = "api-category-grid";
    uncategorized.forEach((category) => grid.append(apiCategory(category, categories.get(category), false)));
    section.append(grid);
    elements.apiLayers.append(section);
  }

  const passthrough = apiPassthrough({
    path: "/yinqiao/v1/upstream/{officialApiPath}",
    methods: ["GET", "POST", "PUT", "PATCH", "DELETE"],
  });
  if (passthrough) elements.apiLayers.append(passthrough);
}

function updateAPICapabilityStatus(payload) {
  const info = payload?.info || {};
  const capabilities = payload?.capabilities || {};
  const liveRoutes = Array.isArray(capabilities.routes) ? capabilities.routes : [];
  const versionParts = [];
  if (info.version) versionParts.push(`API ${info.version}`);
  if (info.apiVersion) versionParts.push(`API ${info.apiVersion}`);
  elements.apiVersion.textContent = versionParts.join(" / ") || "Yinqiao API v1";

  const liveByID = new Map(liveRoutes.map((route) => [route.id, route]));
  let registered = 0;
  let changed = 0;
  let missing = 0;
  for (const item of document.querySelectorAll(".api-route[data-route-id]")) {
    const expected = staticAPIRoutes.find((route) => route.id === item.dataset.routeId);
    const live = liveByID.get(item.dataset.routeId);
    const state = item.querySelector(".route-state");
    state.classList.remove("is-checking", "is-registered", "is-changed", "is-missing", "is-offline");
    if (!live) {
      missing += 1;
      state.classList.add("is-missing");
      state.textContent = "未注册";
      continue;
    }
    if (!expected || live.method !== expected.method || live.path !== expected.path || live.responseMode !== expected.responseMode) {
      changed += 1;
      state.classList.add("is-changed");
      state.textContent = "接口变更";
      continue;
    }
    registered += 1;
    state.classList.add("is-registered");
    state.textContent = "已注册";
  }

  const count = document.querySelector("#apiDetectionCount");
  if (count) {
    count.classList.remove("is-checking", "has-warning", "has-error");
    count.textContent = `${registered}/${staticAPIRoutes.length} 路由已注册`;
    if (missing) count.classList.add("has-error");
    else if (changed) count.classList.add("has-warning");
  }
}

function setAPICapabilityOffline() {
  document.querySelectorAll(".api-route .route-state").forEach((state) => {
    state.classList.remove("is-checking", "is-registered", "is-changed", "is-missing");
    state.classList.add("is-offline");
    state.textContent = "API离线";
  });
  const count = document.querySelector("#apiDetectionCount");
  if (count) {
    count.classList.remove("is-checking", "has-warning");
    count.classList.add("has-error");
    count.textContent = "API离线";
  }
}

async function loadAPICapabilities() {
  try {
    const response = await fetch(endpoints.apiCapabilities, { headers: { Accept: "application/json" }, cache: "no-store" });
    const payload = await readJSON(response);
    if (!response.ok || !payload.ok) throw new Error(payload.error || "无法读取 Yinqiao API 能力声明");
    updateAPICapabilityStatus(payload.data);
  } catch (_) {
    setAPICapabilityOffline();
  }
}

function compatibilityCell(supported, label) {
  const cell = document.createElement("td");
  const mark = document.createElement("span");
  mark.className = `compatibility-mark ${supported ? "supported" : "unavailable"}`;
  mark.textContent = supported ? "✓" : "—";
  mark.title = `${label}：${supported ? "支持" : "未提供"}`;
  mark.setAttribute("aria-label", mark.title);
  cell.append(mark);
  return cell;
}

function renderThirdPartyApps() {
  const root = document.querySelector("#appCompatibilityRows");
  if (!root) return;
  root.replaceChildren();
  const columns = ["android", "iphone", "windows", "mac", "subsonic", "synology", "jellyfin", "ampache"];
  const labels = ["安卓", "iOS", "Win", "Mac", "OpenSubsonic", "Audio Station", "Jellyfin", "Ampache"];
  for (const app of thirdPartyApps) {
    const row = document.createElement("tr");
    const appCell = document.createElement("th");
    appCell.scope = "row";
    const identity = document.createElement("div");
    identity.className = "client-identity";
    const icon = document.createElement("img");
    icon.className = "client-icon";
    icon.src = app.icon;
    icon.alt = "";
    icon.decoding = "async";
    const name = app.source ? document.createElement("a") : document.createElement("strong");
    name.textContent = app.name;
    name.title = `${app.fullName || app.name} · ${app.note}${app.recommended ? " · 推荐" : ""}`;
    if (app.source) {
      name.href = app.source;
      name.target = "_blank";
      name.rel = "noreferrer";
    }
    identity.append(icon, name);
    appCell.append(identity);
    row.append(appCell);
    columns.forEach((column, index) => {
      const supported = index < 4 ? app.platforms.includes(column) : app.protocols.includes(column);
      row.append(compatibilityCell(supported, labels[index]));
    });
    root.append(row);
  }
}

function showMessage(message, type) {
  elements.formMessage.textContent = message;
  elements.formMessage.className = `form-message visible ${type}`;
}

function clearMessage() {
  elements.formMessage.textContent = "";
  elements.formMessage.className = "form-message";
}

function showToast(message, type = "success") {
  window.clearTimeout(toastTimer);
  elements.toast.textContent = message;
  elements.toast.dataset.state = type;
  elements.toast.hidden = false;
  requestAnimationFrame(() => elements.toast.classList.add("is-visible"));
  toastTimer = window.setTimeout(() => {
    elements.toast.classList.remove("is-visible");
    window.setTimeout(() => { elements.toast.hidden = true; }, 180);
  }, 1800);
}

elements.form.addEventListener("submit", async (event) => {
  event.preventDefault();
  clearMessage();
  const username = elements.username.value.trim();
  const password = elements.password.value;
  if (!username || !password) {
    showMessage("请输入飞牛音乐用户名和密码。", "error");
    (!username ? elements.username : elements.password).focus();
    return;
  }

  elements.submitButton.disabled = true;
  elements.submitButton.textContent = "正在验证";
  try {
	const response = await fetch(endpoints.accounts, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ username, password }),
    });
    const payload = await readJSON(response);
    if (!response.ok || !payload.ok) {
      const messages = {
        INVALID_CREDENTIALS: "用户名或密码不正确，请确认该账号可以正常登录飞牛音乐。",
        UPSTREAM_120001: "飞牛音乐登录状态无效，请重新确认账号权限。",
        UPSTREAM_120002: "用户名或密码不正确。",
        UPSTREAM_120003: "账号暂时被限制，请稍后再试。",
        API_UNAVAILABLE: "无法连接本机 Yinqiao API，请检查应用运行状态。",
        CREDENTIAL_STORE_ERROR: "账号验证成功，但加密保存失败，请查看服务日志。",
      };
      throw new Error(messages[payload.code] || payload.error || "绑定失败");
    }
    elements.password.value = "";
	accountFormOpen = false;
    showToast(`账号 ${payload.username} 已绑定`);
	await Promise.all([loadAccounts(), loadStatus()]);
  } catch (error) {
    showMessage(error.message || "绑定失败，请稍后重试。", "error");
  } finally {
    elements.submitButton.disabled = false;
    elements.submitButton.textContent = "验证并绑定";
  }
});

elements.passwordToggle.addEventListener("click", () => {
  const reveal = elements.password.type === "password";
  elements.password.type = reveal ? "text" : "password";
  const label = reveal ? "隐藏密码" : "显示密码";
  elements.passwordToggle.title = label;
  elements.passwordToggle.setAttribute("aria-label", label);
});

elements.refreshButton.addEventListener("click", loadStatus);

elements.addAccountButton.addEventListener("click", () => {
	accountFormOpen = true;
	renderAccounts(currentBoundUsers);
	clearMessage();
	elements.username.value = "";
	elements.password.value = "";
	elements.username.focus();
});

elements.cancelAccountButton.addEventListener("click", () => {
	accountFormOpen = false;
	clearMessage();
	renderAccounts(currentBoundUsers);
});

elements.settingsForm.addEventListener("input", markSettingsDirty);
elements.settingsForm.addEventListener("change", markSettingsDirty);

elements.resetSettingsButton.addEventListener("click", () => {
	elements.protocolPort.value = 14040;
	elements.advertisedHost.value = "";
	elements.toggleSubsonic.checked = true;
	elements.toggleSynology.checked = true;
	elements.toggleJellyfin.checked = true;
	elements.toggleAmpache.checked = true;
	markSettingsDirty();
});

elements.settingsForm.addEventListener("submit", async (event) => {
	event.preventDefault();
	const settings = settingsPayloadFromForm();
	if (!Number.isInteger(settings.protocolPort) || settings.protocolPort < 1024 || settings.protocolPort > 65535) {
		elements.settingsState.textContent = "端口无效";
		elements.settingsState.classList.add("error");
		elements.protocolPort.focus();
		return;
	}
	elements.saveSettingsButton.disabled = true;
	elements.saveSettingsButton.textContent = "正在保存";
	try {
		const response = await fetch(endpoints.settings, {
			method: "PUT",
			headers: { "Content-Type": "application/json", Accept: "application/json" },
			body: JSON.stringify(settings),
		});
		const payload = await readJSON(response);
		if (!response.ok || !payload.ok) throw new Error(payload.error || "设置保存失败");
		renderSettings(payload.settings);
		await Promise.allSettled([loadNetworkAddress(), loadCapabilities(), loadStatus()]);
		showToast("设置已保存并生效");
	} catch (error) {
		elements.settingsState.textContent = "保存失败";
		elements.settingsState.classList.add("error");
		showToast(error.message || "设置保存失败", "error");
	} finally {
		elements.saveSettingsButton.disabled = false;
		elements.saveSettingsButton.textContent = "保存设置";
	}
});

document.querySelectorAll("[data-copy-target]").forEach((button) => {
  button.addEventListener("click", async () => {
    const target = document.querySelector(`#${button.dataset.copyTarget}`);
    if (!target) return;
    try {
      await navigator.clipboard.writeText(target.textContent.trim());
      button.classList.add("is-copied");
      showToast("地址已复制");
      window.setTimeout(() => button.classList.remove("is-copied"), 1200);
    } catch (_) {
      showToast("无法访问剪贴板，请手动复制地址", "error");
    }
  });
});

if (window.matchMedia("(hover: hover) and (pointer: fine)").matches) {
  document.addEventListener("pointermove", (event) => {
    const surface = event.target.closest(".glass-surface, .sidebar");
    if (illuminatedSurface && illuminatedSurface !== surface) {
      illuminatedSurface.style.removeProperty("--glass-x");
      illuminatedSurface.style.removeProperty("--glass-y");
    }
    illuminatedSurface = surface;
    if (!surface) return;
    window.cancelAnimationFrame(illuminationFrame);
    illuminationFrame = window.requestAnimationFrame(() => {
      const bounds = surface.getBoundingClientRect();
      const x = ((event.clientX - bounds.left) / bounds.width) * 100;
      const y = ((event.clientY - bounds.top) / bounds.height) * 100;
      surface.style.setProperty("--glass-x", `${Math.max(0, Math.min(100, x))}%`);
      surface.style.setProperty("--glass-y", `${Math.max(0, Math.min(100, y))}%`);
    });
  }, { passive: true });
  document.addEventListener("pointerleave", () => {
    if (!illuminatedSurface) return;
    illuminatedSurface.style.removeProperty("--glass-x");
    illuminatedSurface.style.removeProperty("--glass-y");
    illuminatedSurface = null;
  });
}

window.addEventListener("hashchange", () => {
  const requested = window.location.hash.slice(1);
  activateView(document.querySelector(`[data-view="${requested}"]`) ? requested : "overview", false);
});

configureMusicLink();
renderAPIManifest(staticAPIRoutes);
renderThirdPartyApps();
const initialView = window.location.hash.slice(1);
activateView(document.querySelector(`[data-view="${initialView}"]`) ? initialView : "overview", false);
Promise.allSettled([loadNetworkAddress(), loadStatus(), loadAccounts(), loadSettings(), loadCapabilities(), loadAPICapabilities()]);
