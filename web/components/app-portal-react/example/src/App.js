import React from 'react';

import { ConvoyApp } from 'convoy-app-react';
import 'convoy-app-react/dist/index.css';

const App = () => {
	return (
		<div>
			<ConvoyApp
				token={'CO.km296UEHbaAQF9X7.18x2VjWNbL8EXyKIBrxZq6XPmyc1zT266CUK7553Or4XnCNM9zjpgErAcxHwSqGg'}
				apiURL={'http://localhost:5005'}
			/>
		</div>
	);
};

export default App;
