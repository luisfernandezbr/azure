import React from 'react';
import ReactDOM from 'react-dom';
import { AppContext } from '@pinpt/agent.websdk';
import App from './App';

ReactDOM.render(
	<React.StrictMode>
		<AppContext.Provider publisher="Pinpoint" refType="azure">
			<App />
		</AppContext.Provider>
	</React.StrictMode>,
	document.getElementById('root')
);
