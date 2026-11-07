import { Show } from 'solid-js';
import { app, setApp, doSetup, doLogin, noUsers, authBooted, pwChecks, t, lang, setLang } from '../store';

/* 首启初始化 / 登录。
 * 多语言：进入即可切换 中/EN（无需登录），选择持久化；
 * 首次进入默认语言按浏览器环境检测（见 i18n/index.ts）。 */
export default function AuthScreen() {
  const submit = (e: SubmitEvent) => { e.preventDefault(); noUsers() ? doSetup() : doLogin(); };

  return (
    <div class="auth-screen">
      <div class="auth-lang">
        <div class="lang-switch" role="group" aria-label={t('aria.lang')}>
          <button type="button" class="lang-btn" classList={{ active: lang() === 'zh' }} onClick={() => setLang('zh')}>中</button>
          <button type="button" class="lang-btn" classList={{ active: lang() === 'en' }} onClick={() => setLang('en')}>EN</button>
        </div>
      </div>
      <div class="auth-card">
        <div class="auth-brand">
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" /><path d="M7 8h2m2 0h2m2 0h2M7 11h10" />
          </svg>
          <div class="auth-brand-name">Dockge</div>
        </div>

        <Show when={authBooted()} fallback={
          <div class="auth-booting" role="status">{t('auth.booting')}</div>
        }>
        <form onSubmit={submit}>
          <h1 class="auth-title">{noUsers() ? t('auth.setupTitle') : t('auth.loginTitle')}</h1>
          <p class="auth-subtitle">{noUsers() ? t('auth.setupSubtitle') : t('auth.loginSubtitle')}</p>

          <div class="form-group">
            <label class="form-label">{t('auth.username')}</label>
            <input class="form-input" value={app.authForm.username} autocomplete="username"
                   onInput={e => setApp('authForm', 'username', e.currentTarget.value)} />
          </div>
          <div class="form-group">
            <label class="form-label">{t('auth.password')}</label>
            <input class="form-input" type="password" value={app.authForm.password}
                   autocomplete={noUsers() ? 'new-password' : 'current-password'}
                   onInput={e => setApp('authForm', 'password', e.currentTarget.value)} />
          </div>

          <Show when={noUsers()}>
            <div class="form-group">
              <label class="form-label">{t('auth.confirmPassword')}</label>
              <input class="form-input" type="password" value={app.authForm.confirm} autocomplete="new-password"
                     onInput={e => setApp('authForm', 'confirm', e.currentTarget.value)} />
            </div>
            <ul class="pw-checklist">
              {pwChecks().map(c => (
                <li classList={{ ok: c.ok }}>
                  <span class="pw-check-dot">{c.ok ? '✓' : '·'}</span><span>{c.label}</span>
                </li>
              ))}
            </ul>
          </Show>

          <Show when={app.authForm.error}>
            <p class="auth-error">{app.authForm.error}</p>
          </Show>

          <button type="submit" class="btn btn-primary auth-submit" disabled={app.authForm.busy}>
            {noUsers() ? t('auth.createAdmin') : (app.authForm.busy ? t('auth.signingIn') : t('auth.signIn'))}
          </button>
        </form>
        </Show>
      </div>
    </div>
  );
}
