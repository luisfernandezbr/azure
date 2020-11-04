import React from 'react';
import { SimulatorInstaller, Integration, IProcessingDetail, IProcessingState, IInstalledLocation } from '@pinpt/agent.websdk';
import IntegrationUI from './integration';

function App() {
	// check to see if we are running local and need to run in simulation mode
	if (window === window.parent && window.location.href.indexOf('localhost') > 0) {
		const integration: Integration = {
			name: 'Azure DevOps',
			description: 'The official Azure DevOps integration for Pinpoint',
			tags: ['Source Code Management', 'Issue Management'],
			installed: false,
			refType: 'azure',
			icon: 'https://pinpoint.com/images/integrations/Azure.svg',
			publisher: {
				name: 'Pinpoint',
				avatar: 'https://pinpoint.com/logo/logomark/blue.png',
				url: 'https://pinpoint.com'
			},
			uiURL: window.location.href
		};

		const processingDetail: IProcessingDetail = {
			createdDate: Date.now() - (86400000 * 5) - 60000,
			processed: true,
			lastProcessedDate: Date.now() - (86400000 * 2),
			lastExportRequestedDate: Date.now() - ((86400000 * 5) + 60000),
			lastExportCompletedDate: Date.now() - (86400000 * 5),
			state: IProcessingState.IDLE,
			throttled: false,
			throttledUntilDate: Date.now() + 2520000,
			paused: false,
			location: IInstalledLocation.CLOUD
		};

		return <SimulatorInstaller integration={integration} processingDetail={processingDetail} />;
	}
	return <IntegrationUI />;
}

export default App;
