import Cookies from 'js-cookie';
import { useTranslation } from 'react-i18next';

export default function ErrorPage() {
  const { t } = useTranslation();
  // Get error from query parameters
  const urlParams = new URLSearchParams(window.location.search);
  const error = urlParams.get('error') || t('error_default');

  return (
    <main className="content">
      <p>{error}</p>
      <a href="/">{t('error_text2')}</a>
    </main>
  );
}
