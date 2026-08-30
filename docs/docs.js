(() => {
  const pages = [
    ["index.html", "Overview"],
    ["getting-started.html", "Getting started"],
    ["language-reference.html", "Language reference"],
    ["concurrency-runtime.html", "Concurrency & runtime"],
    ["standard-library.html", "Standard library"],
    ["tooling.html", "Compiler & tooling"],
    ["implementation.html", "Implementation notes"],
    ["misc.html", "Misc"]
  ];

  const groups = [
    ["Learn", pages.slice(0, 3)],
    ["Runtime & APIs", pages.slice(3, 5)],
    ["Develop Niraluli", pages.slice(5, 7)],
    ["Extras", pages.slice(7)]
  ];

  const current = location.pathname.split("/").pop() || "index.html";
  const nav = groups.map(([title, links]) => `
    <div class="sidebar-title">${title}</div>
    ${links.map(([href, label]) =>
      `<a class="nav-link${href === current ? " active" : ""}" href="${href}">${label}</a>`
    ).join("")}
  `).join("");

  const topbar = document.createElement("header");
  topbar.className = "topbar";
  topbar.innerHTML = `
    <a class="brand" href="index.html">
      <span class="brand-mark">நிரலுளி</span>
      <span>Niraluli docs</span>
      <span class="version">v0.72</span>
    </a>
    <div class="top-actions">
      <button class="icon-button menu-button" type="button" aria-label="Open navigation">☰</button>
      <button class="icon-button theme-button" type="button" aria-label="Toggle color theme">◐</button>
    </div>`;

  const sidebar = document.createElement("aside");
  sidebar.className = "sidebar";
  sidebar.setAttribute("aria-label", "Documentation navigation");
  sidebar.innerHTML = nav;

  const main = document.querySelector("main");
  if (main) {
    const layout = document.createElement("div");
    layout.className = "layout";
    main.parentNode.insertBefore(layout, main);
    layout.append(sidebar, main);
  }

  document.body.prepend(topbar);
  const skip = document.createElement("a");
  skip.className = "skip-link";
  skip.href = "#main-content";
  skip.textContent = "Skip to content";
  document.body.prepend(skip);

  const footer = document.createElement("footer");
  footer.className = "site-footer";
  footer.innerHTML = `Niraluli v0.72 developer documentation · Experimental Linux/C compiler`;
  document.body.append(footer);

  const savedTheme = localStorage.getItem("uli-doc-theme");
  if (savedTheme) document.documentElement.dataset.theme = savedTheme;
  document.querySelector(".theme-button").addEventListener("click", () => {
    const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    localStorage.setItem("uli-doc-theme", next);
  });
  document.querySelector(".menu-button").addEventListener("click", () => {
    document.body.classList.toggle("nav-open");
  });
  document.querySelectorAll(".nav-link").forEach(link => link.addEventListener("click", () => {
    document.body.classList.remove("nav-open");
  }));

  document.querySelectorAll("pre").forEach(pre => {
    const button = document.createElement("button");
    button.className = "copy-button";
    button.type = "button";
    button.textContent = "Copy";
    button.addEventListener("click", async () => {
      const text = pre.querySelector("code")?.textContent || pre.textContent;
      const legacyCopy = () => {
        const area = document.createElement("textarea");
        area.value = text;
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.append(area);
        area.select();
        document.execCommand("copy");
        area.remove();
      };
      if (navigator.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(text);
        } catch {
          legacyCopy();
        }
      } else {
        legacyCopy();
      }
      button.textContent = "Copied";
      setTimeout(() => { button.textContent = "Copy"; }, 1200);
    });
    pre.append(button);
  });
})();
