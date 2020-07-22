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
			icon: `data:image/svg+xml,%3Csvg id='f4337506-5d95-4e80-b7ca-68498c6e008e' data-name='fluent_icons' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' width='18' height='18' viewBox='0 0 18 18'%3E%3Cdefs%3E%3ClinearGradient id='ba420277-700e-42cc-9de9-5388a5c16e54' x1='9' y1='16.97' x2='9' y2='1.03' gradientUnits='userSpaceOnUse'%3E%3Cstop offset='0' stop-color='%230078d4'/%3E%3Cstop offset='0.16' stop-color='%231380da'/%3E%3Cstop offset='0.53' stop-color='%233c91e5'/%3E%3Cstop offset='0.82' stop-color='%23559cec'/%3E%3Cstop offset='1' stop-color='%235ea0ef'/%3E%3C/linearGradient%3E%3C/defs%3E%3Ctitle%3EIcon-devops-261%3C/title%3E%3Cpath id='a91f0ca4-8fb7-4019-9c09-0a52e2c05754' data-name='Path 1' d='M17,4v9.74l-4,3.28-6.2-2.26V17L3.29,12.41l10.23.8V4.44Zm-3.41.49L7.85,1V3.29L2.58,4.84,1,6.87v4.61l2.26,1V6.57Z' fill='url(%23ba420277-700e-42cc-9de9-5388a5c16e54)'/%3E%3C/svg%3E`,
			publisher: {
				name: 'Pinpoint',
				avatar: 'https://avatars0.githubusercontent.com/u/24400526?s=200&v=4',
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
