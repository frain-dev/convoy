import { Story, Meta } from '@storybook/angular/types-6-0';
import { EmptyStateComponent } from '../empty-state/empty-state.component';

export default {
	title: 'Example/EmptyState',
	component: EmptyStateComponent,
	argTypes: {
		imgSrc: {
			control: { type: 'string' }
		},
        heading: {
			control: { type: 'string' }
		},
        description: {
			control: { type: 'string' }
		},
        buttonText: {
			control: { type: 'string' }
		},
		type: {
			options: ['normal', 'table'],
			control: { type: 'select' },
			defaultValue: 'normal'
		},
        class: {
			control: { type: 'string' }
		},
	}
} as Meta;

const Template: Story<EmptyStateComponent> = (args: EmptyStateComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	imgSrc: '/assets/img/empty-state.svg',
	heading: 'heading',
	description: 'description',
	buttonText: 'button text',
	type: 'normal',
    class: 'p-5'
};
