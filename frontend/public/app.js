const authDialog = document.querySelector("#auth-dialog");
const authForm = document.querySelector("#auth-form");
const authMessage = document.querySelector("#auth-message");
const authTitle = document.querySelector("#auth-title");
const authSubtitle = document.querySelector("#auth-subtitle");
const emailInput = document.querySelector("#email");
const passwordInput = document.querySelector("#password");
const authSubmit = document.querySelector("#auth-submit");
const loginTab = document.querySelector("#login-tab");
const registerTab = document.querySelector("#register-tab");
const loginOpen = document.querySelector("#login-open");
const registerOpen = document.querySelector("#register-open");
const userMenu = document.querySelector("#user-menu");
const userAvatar = document.querySelector("#user-avatar");
const userName = document.querySelector("#user-name");
const sidebarAccount = document.querySelector("#sidebar-account");
const sidebarAvatar = document.querySelector("#sidebar-avatar");
const sidebarName = document.querySelector("#sidebar-name");
const views = new Map([...document.querySelectorAll(".view")].map((view) => [view.id, view]));
const navItems = [...document.querySelectorAll(".nav-item")];
const articleList = document.querySelector("#article-list");
const feedMessage = document.querySelector("#feed-message");
const feedCount = document.querySelector("#feed-count");
const feedCaption = document.querySelector("#feed-caption");
const searchInput = document.querySelector("#search-input");
const followingList = document.querySelector("#following-list");
const followingMessage = document.querySelector("#following-message");
const subscriptionsMessage = document.querySelector("#subscriptions-message");
const subscriptionStatus = document.querySelector("#subscription-status");
const profileContent = document.querySelector("#profile-content");
const articleForm = document.querySelector("#article-form");
const articleMessage = document.querySelector("#article-message");
const planNotice = document.querySelector("#plan-notice");
const publishButton = document.querySelector("#publish-button");
const articleDetail = document.querySelector("#article-detail");
const commentsList = document.querySelector("#comments-list");
const commentCount = document.querySelector("#comment-count");
const commentForm = document.querySelector("#comment-form");
const commentInput = document.querySelector("#comment-input");
const commentMessage = document.querySelector("#comment-message");
const commentSignin = document.querySelector("#comment-signin");

let session = readSession();
let authMode = "login";
let currentView = "feed";
let currentArticleID = 0;
let currentProfileID = 0;
let followedAuthorIDs = new Set();
let searchTimer;
let planTimer;

function readSession() {
  try {
    const stored = JSON.parse(sessionStorage.getItem("space-media-session") || "null");
    return stored?.token && stored?.user?.id ? stored : null;
  } catch {
    sessionStorage.removeItem("space-media-session");
    return null;
  }
}

function setMessage(element, text, isError = false) {
  element.textContent = text;
  element.classList.toggle("error", isError);
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (options.body && !(options.body instanceof FormData)) headers.set("Content-Type", "application/json");
  if (session?.token) headers.set("Authorization", `Bearer ${session.token}`);
  const response = await fetch(`/api${path}`, { ...options, headers });
  const contentType = response.headers.get("content-type") || "";
  const result = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    if (response.status === 401 && session?.token) {
      signOut();
      openAuth("login");
      throw new Error("Сессия истекла. Войди в аккаунт снова.");
    }
    const message = typeof result === "string" && !contentType.includes("text/html") ? result.trim() : "";
    throw new Error(message || `Запрос завершился ошибкой (${response.status}).`);
  }
  return result;
}

function openAuth(mode = "login") {
  if (session) return;
  setAuthMode(mode);
  if (!authDialog.open) authDialog.showModal();
}

function setAuthMode(mode) {
  authMode = mode;
  const login = mode === "login";
  loginTab.classList.toggle("active", login);
  registerTab.classList.toggle("active", !login);
  loginTab.setAttribute("aria-selected", String(login));
  registerTab.setAttribute("aria-selected", String(!login));
  authTitle.textContent = login ? "Рады видеть тебя" : "Начни своё путешествие";
  authSubtitle.textContent = login ? "Войди, чтобы продолжить путешествие." : "Создай аккаунт — и публикуй свои истории.";
  authSubmit.innerHTML = login ? 'Войти в аккаунт <span aria-hidden="true">↗</span>' : 'Создать аккаунт <span aria-hidden="true">↗</span>';
  passwordInput.autocomplete = login ? "current-password" : "new-password";
  setMessage(authMessage, "");
}

function showView(name) {
  if (name !== "feed" && !session) {
    openAuth("login");
    return false;
  }
  currentView = name;
  for (const [id, view] of views) view.hidden = id !== `${name}-view`;
  navItems.forEach((item) => item.classList.toggle("active", item.dataset.view === name));
  if (name === "feed") void loadArticles();
  if (name === "following") void loadFollowingFeed();
  if (name === "subscriptions") void loadSubscriptions();
  if (name === "profile") void loadMyProfile();
  if (name === "write") void loadWriterState();
  return true;
}

function renderSession() {
  const authenticated = Boolean(session);
  loginOpen.hidden = authenticated;
  registerOpen.hidden = authenticated;
  userMenu.hidden = !authenticated;
  sidebarAccount.hidden = !authenticated;
  if (!authenticated) return;
  const name = session.user.display_name || session.user.email;
  userName.textContent = name;
  sidebarName.textContent = name;
  userAvatar.textContent = name.slice(0, 1).toUpperCase();
  sidebarAvatar.textContent = name.slice(0, 1).toUpperCase();
  if (session.user.avatar_url) {
    setAvatarImage(userAvatar, session.user.avatar_url, name);
    setAvatarImage(sidebarAvatar, session.user.avatar_url, name);
  }
  void loadFollowedIDs();
}

function signOut() {
  clearInterval(planTimer);
  sessionStorage.removeItem("space-media-session");
  session = null;
  followedAuthorIDs = new Set();
  renderSession();
  if (currentView !== "feed") showView("feed");
}

function setAvatarImage(element, url, name) {
  if (!url) {
    element.textContent = (name || "✦").slice(0, 1).toUpperCase();
    return;
  }
  element.replaceChildren();
  const image = document.createElement("img");
  image.src = url;
  image.alt = `${name} — аватар`;
  image.addEventListener("error", () => setAvatarImage(element, "", name), { once: true });
  element.append(image);
}

function formatDate(value) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", year: "numeric" }).format(new Date(value));
}

function makeAvatar(url, name, sizeClass = "") {
  if (url) {
    const image = document.createElement("img");
    image.className = `avatar ${sizeClass}`.trim();
    image.src = url;
    image.alt = `Аватар: ${name}`;
    image.addEventListener("error", () => {
      image.replaceWith(makeAvatar("", name, sizeClass));
    }, { once: true });
    return image;
  }
  const fallback = document.createElement("span");
  fallback.className = `avatar ${sizeClass}`.trim();
  fallback.textContent = (name || "✦").slice(0, 1).toUpperCase();
  return fallback;
}

function emptyState(text) {
  const element = document.createElement("div");
  element.className = "empty-state";
  element.textContent = text;
  return element;
}

function authorButton(authorID, name, avatarURL) {
  const button = document.createElement("button");
  button.className = "article-author";
  button.type = "button";
  button.dataset.profileId = String(authorID);
  button.append(makeAvatar(avatarURL, name, "avatar-small"));
  const label = document.createElement("span");
  label.textContent = name;
  button.append(label);
  return button;
}

function articleCard(article) {
  const card = document.createElement("article");
  card.className = "article-card";
  const authorLine = document.createElement("div");
  authorLine.append(authorButton(article.author_id, article.author_name, article.author_avatar));
  const date = document.createElement("time");
  date.className = "article-date";
  date.dateTime = article.created_at;
  date.textContent = formatDate(article.created_at);
  authorLine.append(date);
  const title = document.createElement("h2");
  title.className = "article-title";
  title.textContent = article.title;
  const summary = document.createElement("p");
  summary.className = "article-summary";
  summary.textContent = article.summary || "Открой статью, чтобы прочитать историю полностью.";
  const actions = document.createElement("div");
  actions.className = "article-actions";
  actions.append(
    reactionButton(article, "like", "♡", article.likes),
    reactionButton(article, "dislike", "♧", article.dislikes),
  );
  const comments = document.createElement("button");
  comments.className = "reaction-button";
  comments.type = "button";
  comments.textContent = `◌ ${article.comment_count}`;
  comments.setAttribute("aria-label", `${article.comment_count} комментариев`);
  comments.addEventListener("click", () => void openArticle(article.id));
  actions.append(comments);
  const read = document.createElement("button");
  read.className = "reaction-button article-read";
  read.type = "button";
  read.textContent = "Читать статью →";
  read.addEventListener("click", () => void openArticle(article.id));
  actions.append(read);
  card.append(authorLine, title, summary, actions);
  return card;
}

function reactionButton(article, reaction, icon, count) {
  const button = document.createElement("button");
  button.className = `reaction-button ${article.my_reaction === reaction ? "selected" : ""}`;
  button.type = "button";
  button.textContent = `${icon} ${count}`;
  button.setAttribute("aria-label", `${reaction === "like" ? "Нравится" : "Не нравится"}: ${count}`);
  button.addEventListener("click", async () => {
    if (!session) return openAuth();
    button.disabled = true;
    try {
      await request(`/articles/${article.id}/reaction`, {
        method: "POST",
        body: JSON.stringify({ reaction }),
      });
      if (currentArticleID === article.id) await openArticle(article.id);
      else if (currentView === "following") await loadFollowingFeed();
      else if (currentView === "profile") {
        if (session?.user.id === currentProfileID) await loadMyProfile();
        else await loadPublicProfile(currentProfileID);
      }
      else await loadArticles();
    } catch (error) {
      setMessage(currentView === "following" ? followingMessage : feedMessage, error.message, true);
    } finally {
      button.disabled = false;
    }
  });
  return button;
}

async function loadArticles() {
  setMessage(feedMessage, "");
  const query = searchInput.value.trim();
  const caption = query ? `Результаты поиска: «${query}»` : "Свежие публикации сообщества";
  feedCaption.textContent = caption;
  try {
    const articles = await request(`/articles${query ? `?q=${encodeURIComponent(query)}` : ""}`);
    articleList.replaceChildren();
    feedCount.textContent = `${articles.length} ${articles.length === 1 ? "статья" : "статей"}`;
    if (!articles.length) articleList.append(emptyState(query ? "По этому запросу ничего не нашлось. Попробуй другое слово." : "Пока нет статей. Загляни сюда позже или опубликуй первую историю."));
    for (const article of articles) articleList.append(articleCard(article));
  } catch (error) {
    setMessage(feedMessage, error.message, true);
  }
}

async function loadFollowedIDs() {
  if (!session) return;
  try {
    const authors = await request("/follows");
    followedAuthorIDs = new Set(authors.map((author) => author.id));
  } catch (error) {
    console.error("Не удалось загрузить список авторов:", error);
  }
}

async function loadFollowingFeed() {
  if (!session) return openAuth();
  setMessage(followingMessage, "");
  try {
    const articles = await request("/articles?followed=true");
    followingList.replaceChildren();
    if (!articles.length) followingList.append(emptyState("Лента пока пуста. Открой профиль автора в статье и подпишись на него."));
    for (const article of articles) followingList.append(articleCard(article));
  } catch (error) {
    setMessage(followingMessage, error.message, true);
  }
}

async function openArticle(articleID) {
  currentArticleID = articleID;
  currentView = "article";
  for (const view of views.values()) view.hidden = view.id !== "article-view";
  navItems.forEach((item) => item.classList.remove("active"));
  articleDetail.replaceChildren(emptyState("Загружаю статью..."));
  commentsList.replaceChildren();
  commentForm.hidden = !session;
  commentSignin.replaceChildren();
  if (!session) {
    commentSignin.append(document.createTextNode("Чтобы оставить комментарий или реакцию, "));
    const login = document.createElement("button");
    login.className = "text-button";
    login.type = "button";
    login.textContent = "войди в аккаунт";
    login.addEventListener("click", () => openAuth());
    commentSignin.append(login, document.createTextNode("."));
  }
  setMessage(commentMessage, "");
  try {
    const [article, comments] = await Promise.all([
      request(`/articles/${articleID}`),
      request(`/articles/${articleID}/comments`),
    ]);
    renderArticleDetail(article);
    renderComments(comments);
  } catch (error) {
    articleDetail.replaceChildren(emptyState(error.message));
  }
}

function renderArticleDetail(article) {
  articleDetail.replaceChildren();
  const heading = document.createElement("div");
  heading.append(authorButton(article.author_id, article.author_name, article.author_avatar));
  const date = document.createElement("time");
  date.className = "article-date";
  date.textContent = formatDate(article.created_at);
  heading.append(date);
  const title = document.createElement("h1");
  title.className = "article-title";
  title.textContent = article.title;
  const summary = document.createElement("p");
  summary.className = "article-summary";
  summary.textContent = article.summary;
  const body = document.createElement("div");
  body.className = "article-body";
  body.textContent = article.body;
  const actions = document.createElement("div");
  actions.className = "article-actions";
  actions.append(reactionButton(article, "like", "♡", article.likes), reactionButton(article, "dislike", "♧", article.dislikes));
  if (session && session.user.id !== article.author_id) {
    const follow = document.createElement("button");
    follow.className = "button button-quiet";
    follow.type = "button";
    follow.textContent = followedAuthorIDs.has(article.author_id) ? "Отписаться" : "Следить за автором";
    follow.addEventListener("click", async () => {
      follow.disabled = true;
      try {
        await request(`/follows/${article.author_id}`, {
          method: followedAuthorIDs.has(article.author_id) ? "DELETE" : "POST",
        });
        if (followedAuthorIDs.has(article.author_id)) followedAuthorIDs.delete(article.author_id);
        else followedAuthorIDs.add(article.author_id);
        follow.textContent = followedAuthorIDs.has(article.author_id) ? "Отписаться" : "Следить за автором";
      } catch (error) {
        setMessage(commentMessage, error.message, true);
      } finally {
        follow.disabled = false;
      }
    });
    actions.append(follow);
  }
  articleDetail.append(heading, title, summary, body, actions);
}

function renderComments(comments) {
  commentCount.textContent = String(comments.length);
  commentsList.replaceChildren();
  if (!comments.length) commentsList.append(emptyState("Комментариев пока нет. Будь первым, кто начнёт обсуждение."));
  for (const comment of comments) {
    const card = document.createElement("article");
    card.className = "comment-card";
    card.append(makeAvatar(comment.avatar_url, comment.author_name, "avatar-small"));
    const content = document.createElement("div");
    content.className = "comment-main";
    const heading = document.createElement("div");
    heading.className = "comment-head";
    const name = document.createElement("strong");
    name.textContent = comment.author_name;
    const date = document.createElement("time");
    date.textContent = formatDate(comment.created_at);
    heading.append(name, date);
    const body = document.createElement("p");
    body.className = "comment-body";
    body.textContent = comment.body;
    content.append(heading, body);
    card.append(content);
    commentsList.append(card);
  }
}

async function loadMyProfile() {
  if (!session) return openAuth();
  await loadFollowedIDs();
  try {
    const profile = await request("/profile/me");
    currentProfileID = profile.id;
    session.user.display_name = profile.display_name;
    session.user.avatar_url = profile.avatar_url;
    sessionStorage.setItem("space-media-session", JSON.stringify(session));
    renderSession();
    await renderProfile(profile, true);
  } catch (error) {
    profileContent.replaceChildren(emptyState(error.message));
  }
}

async function loadPublicProfile(profileID) {
  currentView = "profile";
  currentProfileID = profileID;
  for (const view of views.values()) view.hidden = view.id !== "profile-view";
  navItems.forEach((item) => item.classList.remove("active"));
  profileContent.replaceChildren(emptyState("Загружаю страницу автора..."));
  try {
    if (session) await loadFollowedIDs();
    const profile = await request(`/profiles/${profileID}`);
    await renderProfile(profile, Boolean(session && session.user.id === profile.id));
  } catch (error) {
    profileContent.replaceChildren(emptyState(error.message));
  }
}

async function renderProfile(profile, isMine) {
  profileContent.replaceChildren();
  const hero = document.createElement("section");
  hero.className = "profile-hero";
  hero.append(makeAvatar(profile.avatar_url, profile.display_name));
  const info = document.createElement("div");
  info.className = "profile-info";
  const title = document.createElement("h1");
  title.textContent = profile.display_name;
  const email = document.createElement("p");
  email.className = "muted";
  email.textContent = isMine ? profile.email : "Автор Орбиты";
  const bio = document.createElement("p");
  bio.className = "bio";
  bio.textContent = profile.bio || "Этот автор пока ничего не рассказал о себе.";
  const stats = document.createElement("div");
  stats.className = "profile-stats";
  stats.textContent = `Статей: ${profile.article_count}　 ·　 Читателей: ${profile.followers_count}　 ·　 Следит за: ${profile.following_count || 0}`;
  info.append(title, email, bio, stats);
  if (isMine) {
    const plan = await request("/publication-plan");
    info.append(makeSubscriptionStatus(plan));
  }
  hero.append(info);
  if (isMine) {
    const logout = document.createElement("button");
    logout.className = "button button-quiet";
    logout.type = "button";
    logout.textContent = "Выйти";
    logout.addEventListener("click", signOut);
    hero.append(logout);
  } else if (session) {
    const follow = document.createElement("button");
    follow.className = "button button-primary";
    follow.type = "button";
    follow.textContent = followedAuthorIDs.has(profile.id) ? "Отписаться" : "Следить";
    follow.addEventListener("click", async () => {
      follow.disabled = true;
      try {
        const wasFollowing = followedAuthorIDs.has(profile.id);
        await request(`/follows/${profile.id}`, { method: wasFollowing ? "DELETE" : "POST" });
        if (wasFollowing) followedAuthorIDs.delete(profile.id);
        else followedAuthorIDs.add(profile.id);
        follow.textContent = wasFollowing ? "Следить" : "Отписаться";
        const fresh = await request(`/profiles/${profile.id}`);
        await renderProfile(fresh, false);
      } catch (error) {
        setMessage(feedMessage, error.message, true);
      } finally {
        follow.disabled = false;
      }
    });
    hero.append(follow);
  }
  profileContent.append(hero);

  if (isMine) {
    const panels = document.createElement("div");
    panels.className = "profile-edit-grid";
    panels.append(createProfileEditor(profile), createAvatarEditor(profile));
    const followingPanel = document.createElement("section");
    followingPanel.className = "panel";
    followingPanel.innerHTML = "<h3>Авторы, за которыми я слежу</h3>";
    const list = document.createElement("div");
    list.className = "followed-list";
    try {
      const authors = await request("/follows");
      if (!authors.length) list.append(emptyState("Пока ты ни на кого не подписан."));
      for (const author of authors) {
        const row = document.createElement("div");
        row.className = "followed-author";
        row.append(makeAvatar(author.avatar_url, author.display_name, "avatar-small"));
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = `${author.display_name} · ${author.article_count} статей`;
        button.addEventListener("click", () => void loadPublicProfile(author.id));
        row.append(button);
        list.append(row);
      }
    } catch (error) {
      list.append(emptyState(error.message));
    }
    followingPanel.append(list);
    panels.append(followingPanel);
    profileContent.append(panels);
  }

  const articlesHeading = document.createElement("div");
  articlesHeading.className = "feed-toolbar";
  const articlesTitle = document.createElement("strong");
  articlesTitle.textContent = isMine ? "Мои статьи" : `Статьи автора ${profile.display_name}`;
  articlesHeading.append(articlesTitle);
  const authorArticles = document.createElement("div");
  authorArticles.className = "article-list";
  profileContent.append(articlesHeading, authorArticles);
  try {
    const articles = await request(`/articles?author_id=${profile.id}`);
    if (!articles.length) authorArticles.append(emptyState("У этого автора пока нет опубликованных статей."));
    for (const article of articles) authorArticles.append(articleCard(article));
  } catch (error) {
    authorArticles.append(emptyState(error.message));
  }
}

function createProfileEditor(profile) {
  const panel = document.createElement("section");
  panel.className = "panel";
  const title = document.createElement("h3");
  title.textContent = "О себе";
  const form = document.createElement("form");
  form.innerHTML = '<label>Имя на сайте</label><input name="display_name" minlength="2" maxlength="60" required><label>Описание</label><textarea name="bio" rows="4" maxlength="1000" placeholder="Расскажи о своих интересах и статьях"></textarea><p class="message" role="status"></p><button class="button button-primary" type="submit">Сохранить профиль</button>';
  form.elements.display_name.value = profile.display_name;
  form.elements.bio.value = profile.bio;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const message = form.querySelector(".message");
    try {
      const updated = await request("/profile/me", {
        method: "PUT",
        body: JSON.stringify({
          display_name: form.elements.display_name.value,
          bio: form.elements.bio.value,
        }),
      });
      session.user.display_name = updated.display_name;
      sessionStorage.setItem("space-media-session", JSON.stringify(session));
      renderSession();
      setMessage(message, "Профиль сохранён.");
      await renderProfile(updated, true);
    } catch (error) {
      setMessage(message, error.message, true);
    }
  });
  panel.append(title, form);
  return panel;
}

function createAvatarEditor(profile) {
  const panel = document.createElement("section");
  panel.className = "panel";
  const title = document.createElement("h3");
  title.textContent = "Фотография профиля";
  const form = document.createElement("form");
  form.innerHTML = '<label>Загрузить аватар JPG, PNG или WebP · до 2 МБ</label><input class="file-input" name="avatar" type="file" accept="image/jpeg,image/png,image/webp" required><p class="message" role="status"></p><button class="button button-quiet" type="submit">Загрузить изображение</button>';
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const file = form.elements.avatar.files[0];
    if (!file) return;
    const message = form.querySelector(".message");
    const data = new FormData();
    data.append("avatar", file);
    try {
      const updated = await request("/profile/me/avatar", { method: "POST", body: data });
      session.user.display_name = updated.display_name;
      session.user.avatar_url = updated.avatar_url;
      sessionStorage.setItem("space-media-session", JSON.stringify(session));
      renderSession();
      setMessage(message, "Аватар обновлён.");
      await renderProfile(updated, true);
    } catch (error) {
      setMessage(message, error.message, true);
    }
  });
  panel.append(title, form);
  return panel;
}

function makeSubscriptionStatus(plan) {
  const subscription = document.createElement("p");
  subscription.className = `subscription-status ${plan.active ? "subscription-active" : plan.pending ? "subscription-pending" : "subscription-inactive"}`;
  const icon = document.createElement("span");
  icon.className = "subscription-icon";
  icon.setAttribute("aria-hidden", "true");
  icon.textContent = plan.active ? "✓" : plan.pending ? "◷" : "×";
  const label = document.createElement("span");
  if (plan.active) {
    const expires = plan.subscription_expires_at
      ? ` до ${new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", year: "numeric" }).format(new Date(plan.subscription_expires_at))}`
      : "";
    label.textContent = `Подписка на публикации активна${expires}`;
  } else {
    label.textContent = plan.pending ? "Заявка на подписку обрабатывается" : "Подписки на публикации нет";
  }
  subscription.append(icon, label);
  return subscription;
}

async function loadSubscriptions() {
  if (!session) return openAuth();
  setMessage(subscriptionsMessage, "");
  try {
    const plan = await request("/publication-plan");
    subscriptionStatus.replaceChildren(makeSubscriptionStatus(plan));
    for (const button of document.querySelectorAll("[data-plan-months]")) {
      button.disabled = plan.pending;
      button.textContent = plan.pending
        ? "Заказ обрабатывается"
        : plan.active
          ? `Продлить на ${button.dataset.planMonths === "1" ? "месяц" : "год"}`
          : `Заказать на ${button.dataset.planMonths === "1" ? "месяц" : "год"}`;
    }
    if (plan.pending) setMessage(subscriptionsMessage, "Заказ уже поступил в обработку. Статус обновится автоматически.");
    if (plan.pending) watchPlanStatus();
  } catch (error) {
    subscriptionStatus.replaceChildren(emptyState(error.message));
    return;
  }
}

async function orderSubscription(durationMonths, button) {
  button.disabled = true;
  try {
    await request("/publication-plan", {
      method: "POST",
      body: JSON.stringify({ duration_months: durationMonths }),
    });
    setMessage(subscriptionsMessage, "Заказ отправлен в RabbitMQ и ожидает обработки.");
    await loadSubscriptions();
    watchPlanStatus();
  } catch (error) {
    setMessage(subscriptionsMessage, error.message, true);
    button.disabled = false;
  }
}

async function loadWriterState() {
  if (!session) return openAuth();
  setMessage(articleMessage, "");
  planNotice.hidden = true;
  articleForm.hidden = false;
  try {
    const [profile, plan] = await Promise.all([request("/profile/me"), request("/publication-plan")]);
    if (plan.active) {
      planNotice.hidden = false;
      planNotice.innerHTML = "<h3>План публикаций активен</h3><p>Ты можешь публиковать неограниченное количество статей.</p>";
      return;
    }
    const remaining = Math.max(0, 3 - profile.article_count);
    if (remaining === 0 || plan.pending) {
      articleForm.hidden = remaining === 0;
      planNotice.hidden = false;
      renderPlanNotice(plan);
    } else {
      planNotice.hidden = true;
      articleForm.hidden = false;
      publishButton.disabled = false;
    }
  } catch (error) {
    articleForm.hidden = true;
    planNotice.hidden = false;
    planNotice.replaceChildren(emptyState(error.message));
  }
}

function renderPlanNotice(plan) {
  planNotice.replaceChildren();
  const title = document.createElement("h3");
  title.textContent = plan.pending ? "Заказ подписки обрабатывается" : "Бесплатный лимит исчерпан";
  const description = document.createElement("p");
  description.textContent = plan.pending
    ? "Заказ подписки отправлен. Подожди завершения обработки."
    : "Три публикации доступны бесплатно. Чтобы публиковать дальше, выбери месячную или годовую подписку.";
  planNotice.append(title, description);
  const button = document.createElement("button");
  button.className = "button button-primary";
  button.type = "button";
  button.textContent = plan.pending ? "Посмотреть статус заказа" : "Перейти к подпискам";
  button.addEventListener("click", () => showView("subscriptions"));
  planNotice.append(button);
  if (plan.pending) watchPlanStatus();
}

function watchPlanStatus() {
  clearInterval(planTimer);
  planTimer = setInterval(async () => {
    if (!session || !["write", "subscriptions"].includes(currentView)) return clearInterval(planTimer);
    try {
      const plan = await request("/publication-plan");
      if (!plan.pending) {
        clearInterval(planTimer);
        if (currentView === "write") {
          await loadWriterState();
          if (plan.active) setMessage(articleMessage, "Подписка активирована. Можно продолжать публикацию.");
        } else {
          await loadSubscriptions();
          if (plan.active) setMessage(subscriptionsMessage, "Подписка активирована.");
        }
      }
    } catch (error) {
      clearInterval(planTimer);
      setMessage(currentView === "subscriptions" ? subscriptionsMessage : articleMessage, error.message, true);
    }
  }, 3000);
}

loginOpen.addEventListener("click", () => openAuth("login"));
registerOpen.addEventListener("click", () => openAuth("register"));
userMenu.addEventListener("click", () => showView("profile"));
document.querySelectorAll("[data-plan-months]").forEach((button) => {
  button.addEventListener("click", () => orderSubscription(Number(button.dataset.planMonths), button));
});
document.querySelector("#auth-close").addEventListener("click", () => authDialog.close());
loginTab.addEventListener("click", () => setAuthMode("login"));
registerTab.addEventListener("click", () => setAuthMode("register"));

authForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  authSubmit.disabled = true;
  setMessage(authMessage, "");
  try {
    const endpoint = authMode === "login" ? "/auth/login" : "/auth/register";
    const result = await request(endpoint, {
      method: "POST",
      body: JSON.stringify({ email: emailInput.value.trim(), password: passwordInput.value }),
    });
    session = result;
    sessionStorage.setItem("space-media-session", JSON.stringify(session));
    const profile = await request("/profile/me");
    session.user.display_name = profile.display_name;
    session.user.avatar_url = profile.avatar_url;
    sessionStorage.setItem("space-media-session", JSON.stringify(session));
    passwordInput.value = "";
    authDialog.close();
    renderSession();
    await showView("feed");
  } catch (error) {
    setMessage(authMessage, error.message, true);
  } finally {
    authSubmit.disabled = false;
  }
});

navItems.forEach((item) => item.addEventListener("click", () => showView(item.dataset.view)));
searchInput.addEventListener("input", () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    if (currentView === "feed") void loadArticles();
    else showView("feed");
  }, 250);
});
searchInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    clearTimeout(searchTimer);
    showView("feed");
  }
});
document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    searchInput.focus();
  }
});

document.addEventListener("click", (event) => {
  const profileButton = event.target.closest("[data-profile-id]");
  if (profileButton) void loadPublicProfile(Number(profileButton.dataset.profileId));
});

document.querySelector("#back-to-feed").addEventListener("click", () => showView("feed"));

articleForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  publishButton.disabled = true;
  setMessage(articleMessage, "");
  const form = new FormData(articleForm);
  try {
    const article = await request("/articles", {
      method: "POST",
      body: JSON.stringify({
        title: form.get("title"),
        summary: form.get("summary"),
        body: form.get("body"),
      }),
    });
    articleForm.reset();
    setMessage(articleMessage, "Статья опубликована.");
    await loadMyProfile();
    await openArticle(article.id);
  } catch (error) {
    setMessage(articleMessage, error.message, true);
    if (error.message.includes("free limit reached")) await loadWriterState();
  } finally {
    publishButton.disabled = false;
  }
});

commentForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const submit = commentForm.querySelector("button[type=submit]");
  submit.disabled = true;
  setMessage(commentMessage, "");
  try {
    await request(`/articles/${currentArticleID}/comments`, {
      method: "POST",
      body: JSON.stringify({ body: commentInput.value }),
    });
    commentInput.value = "";
    const comments = await request(`/articles/${currentArticleID}/comments`);
    renderComments(comments);
  } catch (error) {
    setMessage(commentMessage, error.message, true);
  } finally {
    submit.disabled = false;
  }
});

renderSession();
void loadArticles();
