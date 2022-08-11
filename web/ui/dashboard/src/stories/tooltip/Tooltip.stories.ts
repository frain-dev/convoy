import { Story, Meta } from '@storybook/angular/types-6-0';
import { TooltipComponent } from './tooltip.component';

export default {
	title: 'Example/Tooltip',
	component: TooltipComponent,
	argTypes: {
		position: {
			options: ['left', 'right'],
			control: { type: 'select' },
			defaultValue: 'left'
		},
		size: {
			options: ['sm', 'md'],
			control: { type: 'select' },
			defaultValue: 'md'
		}
	}
} as Meta;

const Template: Story<TooltipComponent> = (args: TooltipComponent) => ({
	props: args
});

export const Base = Template.bind({});
Base.args = {
	position: 'left',
	size: 'sm'
};
