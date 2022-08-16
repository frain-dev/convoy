import { Story, Meta } from '@storybook/angular/types-6-0';
import { EmptyStateComponent } from '../../app/components/empty-state/empty-state.component';

export default {
	title: 'Example/EmptyState',
	component: EmptyStateComponent,
	argTypes: {
		imgSrc: {
			control: { type: 'text' }
		},
        heading: {
			control: { type: 'text' }
		},
        description: {
			control: { type: 'text' }
		},
        buttonText: {
			control: { type: 'text' }
		},
		type: {
			options: ['normal', 'table'],
			control: { type: 'select' },
			defaultValue: 'normal'
		}
	}
} as Meta;

const Template: Story<EmptyStateComponent> = (args: EmptyStateComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	imgSrc: '/assets/img/empty-state.svg',
	heading: 'Convoy empty state heading',
	description: 'Convoy empty state description',
	buttonText: 'button text',
	type: 'normal',
};

export const Table = Template.bind({});
Table.args = {
	imgSrc: '/assets/img/empty-state.svg',
	heading: 'Convoy empty state heading',
	description: 'Convoy empty state description',
	buttonText: 'button text',
	type: 'table',
};
