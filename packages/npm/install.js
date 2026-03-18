const os = require("os");
const fs = require("fs");
const path = require("path");
const https = require("https");
const { execSync } = require("child_process");

const PACKAGE = "@cj-ways/userclone";
const BINARY_NAME = "userclone";
const REPO = "cj-ways/userclone";

function getPlatform() {
  const platform = os.platform();
  switch (platform) {
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    case "win32":
      return "windows";
    default:
      throw new Error(`Unsupported platform: ${platform}`);
  }
}

function getArch() {
  const arch = os.arch();
  switch (arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }
}

function getVersion() {
  const packageJson = require("./package.json");
  return packageJson.version;
}

function getDownloadURL(version, platform, arch) {
  const ext = platform === "windows" ? ".exe" : "";
  const filename = `${BINARY_NAME}_${platform}_${arch}${ext}`;
  return `https://github.com/${REPO}/releases/download/v${version}/${filename}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 5) {
        reject(new Error("Too many redirects"));
        return;
      }

      https
        .get(url, (response) => {
          if (
            response.statusCode >= 300 &&
            response.statusCode < 400 &&
            response.headers.location
          ) {
            follow(response.headers.location, redirects + 1);
            return;
          }

          if (response.statusCode !== 200) {
            reject(
              new Error(
                `Download failed with status ${response.statusCode}: ${url}`
              )
            );
            return;
          }

          const file = fs.createWriteStream(dest);
          response.pipe(file);
          file.on("finish", () => {
            file.close(resolve);
          });
          file.on("error", (err) => {
            fs.unlinkSync(dest);
            reject(err);
          });
        })
        .on("error", reject);
    };

    follow(url);
  });
}

async function main() {
  try {
    const platform = getPlatform();
    const arch = getArch();
    const version = getVersion();

    const binDir = path.join(__dirname, "bin");
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    const ext = platform === "windows" ? ".exe" : "";
    const binPath = path.join(binDir, `${BINARY_NAME}${ext}`);

    // Skip if binary already exists and is the right version
    if (fs.existsSync(binPath)) {
      try {
        const currentVersion = execSync(`"${binPath}" --version`, {
          encoding: "utf-8",
        }).trim();
        if (currentVersion.endsWith(version)) {
          console.log(`${PACKAGE} v${version} already installed`);
          return;
        }
      } catch {
        // Version check failed, re-download
      }
    }

    const url = getDownloadURL(version, platform, arch);
    console.log(`Downloading ${PACKAGE} v${version} for ${platform}/${arch}...`);

    await download(url, binPath);

    // Make binary executable on Unix
    if (platform !== "windows") {
      fs.chmodSync(binPath, 0o755);
    }

    console.log(`${PACKAGE} v${version} installed successfully`);
  } catch (err) {
    console.error(`Failed to install ${PACKAGE}: ${err.message}`);
    console.error(
      "You can download the binary manually from:"
    );
    console.error(`https://github.com/${REPO}/releases`);
    process.exit(1);
  }
}

main();
