import { Story, Meta } from '@storybook/angular/types-6-0';
import { BadgeComponent } from '../../app/components/badge/badge.component';

export default {
	title: 'Example/Badge',
	component: BadgeComponent,
	argTypes: {
		text: {
			control: { type: 'text' }
		},
		texture: {
			options: ['deep', 'light'],
			control: { type: 'select' },
			defaultValue: 'light'
		}
	}
} as Meta;

const Template: Story<BadgeComponent> = (args: BadgeComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	text: 'org a',
	texture: 'light'
};
