import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';

export default function ValidatePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { phoneNumber } = useAppContext();

  const enroll = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
   
  };

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t('validate_header')}</h1>
        </header>
        <main>
          <div className="sms-form">

            <p>{t('validate_explanation')}</p>

            {phoneNumber}
          </div>
        </main>
        <footer>
          <div className="actions">
            <Link to="/" id="back-button">
                {t('back')}
            </Link>
            <button id="submit-button" >{t('confirm')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
