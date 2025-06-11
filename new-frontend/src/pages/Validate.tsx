import { useTranslation } from 'react-i18next';
import { useNavigate } from "react-router-dom";

export default function ValidatePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const enroll = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
   
  };

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t('index_header')}</h1>
        </header>
        <main>
          <div id="idin-form">

            <p>{t('enroll_received_attributes')}</p>

            <p>{t('enroll_derived_attributes')}</p>
            
          </div>
        </main>
        <footer>
          <div className="actions">
            <div></div>
            <button id="submit-button" >{t('enroll_load_button')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
