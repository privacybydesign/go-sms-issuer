import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';

export default function ErrorPage() {
  const { t, i18n } = useTranslation();
  // Get error from query parameters
  const urlParams = new URLSearchParams(window.location.search);
  const error = urlParams.get('error') || t('error_default');

  return (
    <>
      <div id="container">
        <header>
          <h1>{t('error_header')}</h1>
        </header>
        <main>
          <div className="sms-form">
            <div className="imageContainer">
              <img src="/images/fail.png" alt="error" />
              <p>{error}</p>
            </div>
          </div>
        </main>
        <footer>
          <div className="actions">
            <Link to={`/${i18n.language}`} id="back-button">
              {t('back')}
            </Link>
            <div></div>
          </div>
        </footer>
      </div>
    </>
  );
}
