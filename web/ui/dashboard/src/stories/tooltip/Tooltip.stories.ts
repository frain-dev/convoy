import { Story, Meta } from '@storybook/angular/types-6-0';
import { TooltipComponent } from '../../app/components/tooltip/tooltip.component';

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
		},
		img: {
			control: { type: 'text' }
		}
	}
} as Meta;

const Template: Story<TooltipComponent> = (args: TooltipComponent) => ({
	props: args,
	template: `<div class="flex justify-center items-center h-200px"><span class="mr-2">click here </span> <convoy-tooltip [size]="size" [position]="position" [img]="img">{{ngContent}}</convoy-tooltip></div>`
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'Convoy tooltip',
	position: 'left',
	size: 'sm',
	img: '/assets/img/small-info-icon.svg'
} as Partial<TooltipComponent>;
